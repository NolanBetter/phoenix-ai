package backend

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Message is one turn in a conversation.
type Message struct {
	Role         string       `json:"role"` // user or assistant
	Content      string       `json:"content"`
	Attachments  []Attachment `json:"attachments,omitempty"`
	Thinking     string       `json:"thinking,omitempty"`
	ModelID      string       `json:"modelId,omitempty"`
	ModelName    string       `json:"modelName,omitempty"`
	TokensPerSec float64      `json:"tps,omitempty"`
	Stopped      bool         `json:"stopped,omitempty"`
	Error        string       `json:"error,omitempty"`
	Time         int64        `json:"time"`
}

// Chat is a saved conversation (chats/<id>.json).
type Chat struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Created  int64     `json:"created"`
	Updated  int64     `json:"updated"`
	Unread   bool      `json:"unread,omitempty"`
	Messages []Message `json:"messages"`
}

// ChatSummary is what the sidebar shows.
type ChatSummary struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Updated int64  `json:"updated"`
	Unread  bool   `json:"unread"` // an answer finished while you were somewhere else
}

// SetViewingChat tells Phoenix which chat is on screen ("" for none).
// Opening a chat marks it read.
func (a *App) SetViewingChat(id string) {
	a.viewMu.Lock()
	a.viewing = id
	a.viewMu.Unlock()
	if id == "" {
		return
	}
	ch, err := a.chats.load(id)
	if err != nil || !ch.Unread {
		return
	}
	ch.Unread = false
	if a.chats.save(ch) == nil {
		a.emit("chats:changed", nil)
	}
}

func (a *App) viewingChat() string {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	return a.viewing
}

type chatStore struct {
	dir string
	mu  sync.Mutex
}

func newChatStore(dir string) *chatStore { return &chatStore{dir: dir} }

var chatIDPattern = regexp.MustCompile(`^[a-f0-9]{16}$`)

func newChatID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *chatStore) path(id string) (string, error) {
	if !chatIDPattern.MatchString(id) {
		return "", errors.New("that chat doesn't exist")
	}
	return filepath.Join(c.dir, id+".json"), nil
}

func (c *chatStore) load(id string) (*Chat, error) {
	p, err := c.path(id)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, errors.New("that chat doesn't exist")
	}
	var ch Chat
	if err := json.Unmarshal(b, &ch); err != nil {
		return nil, errors.New("that chat file is damaged")
	}
	return &ch, nil
}

func (c *chatStore) save(ch *Chat) error {
	p, err := c.path(ch.ID)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return writeJSONAtomic(p, ch)
}

// ListChats returns all chats, newest first.
func (a *App) ListChats() []ChatSummary {
	out := []ChatSummary{}
	entries, err := os.ReadDir(a.paths.Chats)
	if err != nil {
		return out
	}
	for _, e := range entries {
		id := strings.TrimSuffix(e.Name(), ".json")
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || !chatIDPattern.MatchString(id) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.paths.Chats, e.Name()))
		if err != nil {
			continue
		}
		var s ChatSummary
		if json.Unmarshal(b, &s) == nil && s.ID == id {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out
}

// GetChat returns one chat with all its messages.
func (a *App) GetChat(id string) (*Chat, error) { return a.chats.load(id) }

// DeleteChat removes a chat for good.
func (a *App) DeleteChat(id string) error {
	a.genMu.Lock()
	if a.gen != nil && a.gen.chatID == id {
		a.gen.cancel()
	}
	a.genMu.Unlock()
	p, err := a.chats.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	os.RemoveAll(filepath.Join(a.paths.Chats, id)) // its files
	a.emit("chats:changed", nil)
	return nil
}

// RenameChat changes a chat's title.
func (a *App) RenameChat(id, title string) error {
	ch, err := a.chats.load(id)
	if err != nil {
		return err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("the name can't be empty")
	}
	ch.Title = truncate(title, 80)
	if err := a.chats.save(ch); err != nil {
		return err
	}
	a.emit("chats:changed", nil)
	return nil
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	cut := string(r[:n])
	if i := strings.LastIndex(cut, " "); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}

func titleFrom(text string) string {
	line := strings.TrimSpace(strings.SplitN(strings.TrimSpace(text), "\n", 2)[0])
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		return "New chat"
	}
	return truncate(line, 42)
}

// generation is the answer currently being written.
type generation struct {
	chatID string
	cancel context.CancelFunc
}

// SendMessage adds the user's message to a chat (a new one if chatID is
// empty) and starts the answer. attachmentIDs are files added to the message
// box. The answer streams in as "chat:delta" events and finishes with
// "chat:done". Returns the chat ID.
func (a *App) SendMessage(chatID, text, modelID string, think bool, attachmentIDs []string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" && len(attachmentIDs) == 0 {
		return "", errors.New("type a message first")
	}
	if len(attachmentIDs) > MaxAttachments {
		return "", fmt.Errorf("you can add up to %d files to one message", MaxAttachments)
	}
	path, err := a.modelPath(modelID)
	if err != nil {
		return "", err
	}
	if a.generating() {
		return "", errors.New("wait for the current answer to finish, or stop it first")
	}
	var files []Attachment
	for _, id := range attachmentIDs {
		att, ok := a.pendingAttachment(id)
		if !ok {
			return "", errors.New("one of the files is no longer there. Remove it and add it again")
		}
		files = append(files, att)
	}
	if err := a.checkImages(modelID, path, files); err != nil {
		return "", err
	}

	var ch *Chat
	now := time.Now().UnixMilli()
	if chatID == "" {
		if len(a.ListChats()) >= MaxChats {
			return "", fmt.Errorf("you already have %d chats, the most Phoenix keeps. Delete one to start a new chat", MaxChats)
		}
		title := titleFrom(text)
		if text == "" {
			title = titleFrom(files[0].Name)
		}
		ch = &Chat{ID: newChatID(), Title: title, Created: now}
	} else {
		if ch, err = a.chats.load(chatID); err != nil {
			return "", err
		}
	}
	atts, err := a.takePending(ch.ID, attachmentIDs)
	if err != nil {
		return "", err
	}
	ch.Messages = append(ch.Messages, Message{Role: "user", Content: text, Attachments: atts, Time: now})
	ch.Updated = now
	if err := a.chats.save(ch); err != nil {
		return "", fmt.Errorf("couldn't save the chat: %w", err)
	}
	_ = a.updateSettings(func(s *Settings) { s.LastModel = modelID; s.Thinking = think })
	a.emit("chats:changed", nil)
	a.startGeneration(ch, modelID, think)
	return ch.ID, nil
}

// checkImages makes sure the model can see the pictures in a message, and
// that they fit in its memory.
func (a *App) checkImages(modelID, path string, files []Attachment) error {
	tokens, images := 0, 0
	for _, f := range files {
		if f.Kind == attachmentKindImage {
			images++
			tokens += imageTokens(f)
		}
	}
	if images == 0 {
		return nil
	}
	if findVisionFile(path) == "" {
		name := a.modelName(modelID)
		parts := strings.SplitN(modelID, "/", 2)
		if c, ok := catalogByRepo(repoFromDir(parts[0])); ok && c.Vision != "" {
			return fmt.Errorf("%s needs its image support to see pictures. Add it on the Models page, then send again", name)
		}
		return fmt.Errorf("%s can't see pictures. Pick a model marked “Sees images”, or remove the picture", name)
	}
	if ctx := a.getSettings().ContextSize; tokens > ctx-1536 {
		if images == 1 {
			return errors.New("that picture is too much for the model's memory. Raise Memory length in Settings and try again")
		}
		return errors.New("those pictures are too much for the model's memory at once. Send fewer, or raise Memory length in Settings")
	}
	return nil
}

// Regenerate writes a new answer to the last message in a chat.
func (a *App) Regenerate(chatID, modelID string, think bool) error {
	if a.generating() {
		return errors.New("wait for the current answer to finish, or stop it first")
	}
	if _, err := a.modelPath(modelID); err != nil {
		return err
	}
	ch, err := a.chats.load(chatID)
	if err != nil {
		return err
	}
	for len(ch.Messages) > 0 && ch.Messages[len(ch.Messages)-1].Role == "assistant" {
		ch.Messages = ch.Messages[:len(ch.Messages)-1]
	}
	if len(ch.Messages) == 0 {
		return errors.New("there's nothing to answer in this chat")
	}
	path, _ := a.modelPath(modelID)
	if err := a.checkImages(modelID, path, ch.Messages[len(ch.Messages)-1].Attachments); err != nil {
		return err
	}
	if err := a.chats.save(ch); err != nil {
		return err
	}
	a.startGeneration(ch, modelID, think)
	return nil
}

// StopGeneration stops the answer being written. What was written so far is kept.
func (a *App) StopGeneration() {
	a.genMu.Lock()
	defer a.genMu.Unlock()
	if a.gen != nil {
		a.gen.cancel()
	}
}

// IsGenerating tells the window whether an answer is being written.
func (a *App) IsGenerating() string {
	a.genMu.Lock()
	defer a.genMu.Unlock()
	if a.gen == nil {
		return ""
	}
	return a.gen.chatID
}

func (a *App) generating() bool {
	a.genMu.Lock()
	defer a.genMu.Unlock()
	return a.gen != nil
}

type chatDelta struct {
	ChatID   string `json:"chatId"`
	Content  string `json:"content,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

type chatDone struct {
	ChatID  string  `json:"chatId"`
	Message Message `json:"message"`
}

func (a *App) startGeneration(ch *Chat, modelID string, think bool) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.genMu.Lock()
	a.gen = &generation{chatID: ch.ID, cancel: cancel}
	a.genMu.Unlock()

	name := a.modelName(modelID)
	a.emit("chat:start", map[string]string{"chatId": ch.ID, "modelId": modelID, "modelName": name})

	go func() {
		defer cancel()
		msg := a.generate(ctx, ch, modelID, think)
		msg.ModelID, msg.ModelName, msg.Time = modelID, name, time.Now().UnixMilli()

		// Reload in case the chat was renamed meanwhile, then append the answer.
		if latest, err := a.chats.load(ch.ID); err == nil {
			ch = latest
			ch.Messages = append(ch.Messages, msg)
			ch.Updated = msg.Time
			ch.Unread = a.viewingChat() != ch.ID
			_ = a.chats.save(ch)
		}
		a.genMu.Lock()
		a.gen = nil
		a.genMu.Unlock()
		a.emit("chat:done", chatDone{ChatID: ch.ID, Message: msg})
		a.emit("chats:changed", nil)
		switch {
		case msg.Error != "":
			a.notifyIfAway("Phoenix couldn't answer", msg.Error, "chat:"+ch.ID)
		case !msg.Stopped:
			body := previewText(msg.Content, 120)
			if body == "" {
				body = "The answer is ready."
			}
			a.notifyIfAway("New answer in “"+ch.Title+"”", body, "chat:"+ch.ID)
		}
	}()
}

type apiMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // a string, or []contentPart when there are pictures
}

type contentPart struct {
	Type     string    `json:"type"` // text or image_url
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// attachmentSource loads the files sent in a chat.
type attachmentSource interface {
	attachmentText(att Attachment) string
	attachmentImage(att Attachment) (dataURL string, ok bool)
}

type chatFiles struct {
	a      *App
	chatID string
}

func (c chatFiles) attachmentText(att Attachment) string {
	return c.a.readAttachmentText(c.chatID, att)
}
func (c chatFiles) attachmentImage(att Attachment) (string, bool) {
	return c.a.readAttachmentImage(c.chatID, att)
}

// About 3.5 characters of text make one token.
const charsPerToken = 3.5

// imageTokens guesses how much of the model's memory a picture uses.
func imageTokens(att Attachment) int {
	t := att.Width * att.Height / 1024
	if t < 300 {
		t = 300
	}
	return t
}

type promptFile struct {
	name, text string
	truncated  bool // Phoenix only kept the start of the file when it was added
	cut        bool // cut here to fit the model's memory
}

type promptTurn struct {
	role   string
	text   string
	files  []promptFile
	images []Attachment
}

func (t *promptTurn) cost(vision bool) int {
	n := len(t.text)
	for _, f := range t.files {
		n += len(f.text) + len(f.name) + 40
	}
	for _, img := range t.images {
		if vision {
			n += int(float64(imageTokens(img)) * charsPerToken)
		} else {
			n += len(img.Name) + 80
		}
	}
	return n
}

// buildPrompt turns the chat into the message list the engine expects,
// dropping the oldest turns if the conversation no longer fits. Files are
// included as text; pictures are sent as images if the model can see them.
func buildPrompt(system string, msgs []Message, ctxSize int, vision bool, files attachmentSource) []apiMessage {
	var turns []promptTurn
	for _, m := range msgs {
		if m.Error != "" || (strings.TrimSpace(m.Content) == "" && len(m.Attachments) == 0) {
			continue
		}
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		t := promptTurn{role: m.Role, text: m.Content}
		for _, att := range m.Attachments {
			if att.Kind == attachmentKindImage {
				t.images = append(t.images, att)
				continue
			}
			text := "(Phoenix couldn't find this file anymore.)"
			if files != nil {
				text = files.attachmentText(att)
			}
			t.files = append(t.files, promptFile{name: att.Name, text: text, truncated: att.Truncated})
		}
		turns = append(turns, t)
	}
	// Leave room for the answer.
	budget := int(float64(ctxSize-1536) * charsPerToken)
	if budget < 2000 {
		budget = 2000
	}
	budget -= len(system)
	total := 0
	start := len(turns)
	for i := len(turns) - 1; i >= 0; i-- {
		total += turns[i].cost(vision)
		if total > budget && i < len(turns)-1 {
			break
		}
		start = i
	}
	turns = turns[start:]
	// A conversation should start with the user.
	for len(turns) > 1 && turns[0].role != "user" {
		turns = turns[1:]
	}
	// The newest message is always sent. If its files are too long, cut them.
	if n := len(turns); n > 0 && turns[n-1].cost(vision) > budget && len(turns[n-1].files) > 0 {
		last := &turns[n-1]
		fileChars := 0
		for _, f := range last.files {
			fileChars += len(f.text)
		}
		fitFiles(last.files, budget-(last.cost(vision)-fileChars))
	}

	out := []apiMessage{}
	if strings.TrimSpace(system) != "" {
		out = append(out, apiMessage{Role: "system", Content: system})
	}
	for _, t := range turns {
		out = append(out, t.render(vision, files))
	}
	return out
}

// fitFiles shortens the files so their text adds up to at most room
// characters, sharing the space fairly: short files stay whole.
func fitFiles(fs []promptFile, room int) {
	if room < 0 {
		room = 0
	}
	order := make([]int, len(fs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(x, y int) bool { return len(fs[order[x]].text) < len(fs[order[y]].text) })
	for k, i := range order {
		share := room / (len(order) - k)
		if len(fs[i].text) > share {
			cut := share
			for cut > 0 && !utf8.RuneStart(fs[i].text[cut]) {
				cut--
			}
			fs[i].text = fs[i].text[:cut]
			fs[i].cut = true
		}
		room -= len(fs[i].text)
	}
}

func (t promptTurn) render(vision bool, files attachmentSource) apiMessage {
	var b strings.Builder
	for _, f := range t.files {
		name := strings.NewReplacer(`"`, "'", "\n", " ").Replace(f.name)
		fmt.Fprintf(&b, "<file name=\"%s\">\n%s", name, strings.TrimRight(f.text, "\n"))
		switch {
		case f.cut:
			b.WriteString("\n[Phoenix cut the rest of this file because it doesn't fit in the model's memory.]")
		case f.truncated:
			b.WriteString("\n[This file was longer. Phoenix kept only the first part.]")
		}
		b.WriteString("\n</file>\n\n")
	}
	var parts []contentPart
	for _, img := range t.images {
		if vision && files != nil {
			if url, ok := files.attachmentImage(img); ok {
				parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{URL: url}})
				continue
			}
			fmt.Fprintf(&b, "[Picture %q is missing, so it can't be shown.]\n\n", img.Name)
			continue
		}
		fmt.Fprintf(&b, "[The user attached a picture, %q, but this model can't see pictures.]\n\n", img.Name)
	}
	b.WriteString(t.text)
	text := strings.TrimSpace(b.String())
	if len(parts) == 0 {
		return apiMessage{Role: t.role, Content: text}
	}
	if text != "" {
		parts = append(parts, contentPart{Type: "text", Text: text})
	}
	return apiMessage{Role: t.role, Content: parts}
}

type ensureResult struct {
	base string
	err  error
}

func (a *App) generate(ctx context.Context, ch *Chat, modelID string, think bool) Message {
	msg := Message{Role: "assistant"}

	// Load the model. If the user hits Stop while it loads, stop waiting
	// but let it finish loading in the background.
	loaded := make(chan ensureResult, 1)
	go func() {
		b, err := a.runner.ensure(a.ctx, modelID)
		loaded <- ensureResult{b, err}
	}()
	var base string
	select {
	case <-ctx.Done():
		msg.Stopped = true
		return msg
	case res := <-loaded:
		if res.err != nil {
			msg.Error = res.err.Error()
			return msg
		}
		base = res.base
	}

	s := a.getSettings()
	vision := a.runner.hasVision()
	if n := len(ch.Messages); !vision && n > 0 {
		for _, att := range ch.Messages[n-1].Attachments {
			if att.Kind == attachmentKindImage {
				msg.Error = "The model's image support didn't load, so it can't see your picture. Try a smaller model, or close other programs and try again."
				return msg
			}
		}
	}
	body := map[string]any{
		"model":                "phoenix",
		"messages":             buildPrompt(s.SystemPrompt, ch.Messages, s.ContextSize, vision, chatFiles{a, ch.ID}),
		"stream":               true,
		"chat_template_kwargs": map[string]any{"enable_thinking": think},
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		msg.Error = err.Error()
		return msg
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.runner.apiKey())
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			msg.Stopped = true
		} else {
			msg.Error = "Couldn't reach the AI engine. It may have stopped. Try again."
		}
		return msg
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		msg.Error = engineHTTPError(resp.StatusCode, b)
		return msg
	}

	var content, thinking strings.Builder
	var pendC, pendT strings.Builder
	split := &thinkSplitter{}
	lastFlush := time.Now()
	flush := func() {
		if pendC.Len() == 0 && pendT.Len() == 0 {
			return
		}
		a.emit("chat:delta", chatDelta{ChatID: ch.ID, Content: pendC.String(), Thinking: pendT.String()})
		pendC.Reset()
		pendT.Reset()
		lastFlush = time.Now()
	}
	add := func(t, c string) {
		thinking.WriteString(t)
		content.WriteString(c)
		pendT.WriteString(t)
		pendC.WriteString(c)
		if time.Since(lastFlush) > 40*time.Millisecond {
			flush()
		}
	}

	rd := bufio.NewReaderSize(resp.Body, 64<<10)
	var streamErr error
	for {
		line, err := rd.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						Reasoning        string `json:"reasoning"`
					} `json:"delta"`
				} `json:"choices"`
				Timings *struct {
					PredictedPerSecond float64 `json:"predicted_per_second"`
				} `json:"timings"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil {
				if chunk.Error != nil {
					streamErr = errors.New(chunk.Error.Message)
					break
				}
				for _, c := range chunk.Choices {
					t := c.Delta.ReasoningContent + c.Delta.Reasoning
					tt, cc := split.feed(c.Delta.Content)
					add(t+tt, cc)
				}
				if chunk.Timings != nil && chunk.Timings.PredictedPerSecond > 0 {
					msg.TokensPerSec = chunk.Timings.PredictedPerSecond
				}
			}
		}
		if err != nil {
			if err != io.EOF {
				streamErr = err
			}
			break
		}
	}
	tt, cc := split.finish()
	add(tt, cc)
	flush()

	msg.Content = strings.TrimSpace(content.String())
	msg.Thinking = strings.TrimSpace(thinking.String())
	switch {
	case ctx.Err() != nil:
		msg.Stopped = true
	case streamErr != nil:
		msg.Error = "The answer was cut off: " + streamErr.Error()
	case msg.Content == "" && msg.Thinking == "":
		msg.Error = "The model didn't write anything. Try again or pick another model."
	}
	return msg
}

func engineHTTPError(status int, body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	m := strings.ToLower(e.Error.Message)
	switch {
	case strings.Contains(m, "context") && (strings.Contains(m, "exceed") || strings.Contains(m, "too long")):
		return "This chat is too long for the model's memory. Start a new chat, or raise Memory length in Settings."
	case e.Error.Message != "":
		return "The engine couldn't answer: " + e.Error.Message
	}
	return fmt.Sprintf("The engine couldn't answer (status %d).", status)
}

// thinkSplitter separates <think>…</think> text from the answer, for
// models whose thinking the engine doesn't pull out on its own.
type thinkSplitter struct {
	state int // 0 = start, 1 = inside think, 2 = answer
	buf   string
}

const (
	openTag  = "<think>"
	closeTag = "</think>"
)

func (t *thinkSplitter) feed(s string) (thinking, content string) {
	if s == "" {
		return "", ""
	}
	switch t.state {
	case 2:
		return "", s
	case 0:
		t.buf += s
		trimmed := strings.TrimLeft(t.buf, " \t\r\n")
		if len(trimmed) < len(openTag) && strings.HasPrefix(openTag, trimmed) {
			return "", "" // could still become <think>
		}
		if strings.HasPrefix(trimmed, openTag) {
			t.state = 1
			t.buf = ""
			return t.feed(trimmed[len(openTag):])
		}
		t.state = 2
		out := t.buf
		t.buf = ""
		return "", out
	default: // inside think
		t.buf += s
		if i := strings.Index(t.buf, closeTag); i >= 0 {
			thinking = t.buf[:i]
			rest := strings.TrimLeft(t.buf[i+len(closeTag):], " \t\r\n")
			t.buf = ""
			t.state = 2
			return thinking, rest
		}
		// Hold back anything that might be the start of </think>.
		keep := 0
		for k := len(closeTag) - 1; k > 0; k-- {
			if strings.HasSuffix(t.buf, closeTag[:k]) {
				keep = k
				break
			}
		}
		thinking = t.buf[:len(t.buf)-keep]
		t.buf = t.buf[len(t.buf)-keep:]
		return thinking, ""
	}
}

func (t *thinkSplitter) finish() (thinking, content string) {
	defer func() { t.buf = "" }()
	switch t.state {
	case 0:
		return "", t.buf
	case 1:
		return t.buf, ""
	}
	return "", ""
}
