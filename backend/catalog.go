package backend

// CatalogModel is a model Phoenix recommends. All of these are open models
// with permissive licenses, downloaded straight from Hugging Face.
type CatalogModel struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Maker       string   `json:"maker"`
	Params      string   `json:"params"`
	Description string   `json:"description"`
	Repo        string   `json:"repo"`
	File        string   `json:"file"`
	Size        int64    `json:"size"`
	Tags        []string `json:"tags"`
	License     string   `json:"license"`
	// Vision is the model's image-support file in the same repository
	// (llama.cpp calls it an mmproj). Empty for text-only models.
	Vision     string `json:"vision,omitempty"`
	VisionSize int64  `json:"visionSize,omitempty"`
	// Kind is what the model does (see the Kind* constants). Empty means chat.
	Kind string `json:"kind,omitempty"`
	// Revision pins the Hugging Face commit to download, when the engine
	// needs a particular version of the file. Empty means the latest.
	Revision string `json:"-"`
	// Parts are other files the model needs, downloaded along with it.
	Parts []ModelPart `json:"parts,omitempty"`
	// Archive, when set, is downloaded and unpacked instead of a single
	// file from Hugging Face. File is then the file inside it that shows
	// the model is installed.
	Archive *ArchiveSource `json:"-"`
}

// What a model does.
const (
	KindChat     = "chat"     // talks with you (llama.cpp)
	KindImage    = "image"    // makes pictures (stable-diffusion.cpp)
	KindVideo    = "video"    // makes short videos (stable-diffusion.cpp)
	KindListen   = "listen"   // turns speech into text (NeMo-Speech.cpp)
	KindSpeakers = "speakers" // tells apart who is talking (NeMo-Speech.cpp)
	KindVoice    = "voice"    // reads text aloud (sherpa-onnx with Kokoro)
)

// ModelPart is another file a model needs, such as a picture model's
// decoder or a video model's text reader.
type ModelPart struct {
	Role string `json:"role"` // vae, text_encoder
	Name string `json:"name"` // shown while it downloads
	Repo string `json:"repo"`
	File string `json:"file"`
	Size int64  `json:"size"`
}

// ArchiveSource is a .tar.bz2 or .zip that unpacks into the model's folder.
type ArchiveSource struct {
	URL  string
	Size int64
	// Folder is the top folder inside the archive.
	Folder string
}

// kindOf returns a catalog model's kind, with chat as the default.
func (m CatalogModel) kindOf() string {
	if m.Kind == "" {
		return KindChat
	}
	return m.Kind
}

// countsTowardLimit: chat, picture and video models count toward the
// limit of MaxModels. The small voice helpers don't.
func countsTowardLimit(kind string) bool {
	return kind == "" || kind == KindChat || kind == KindImage || kind == KindVideo
}

// totalSize is everything the model downloads: itself, its parts and its
// image support.
func (m CatalogModel) totalSize() int64 {
	n := m.Size + m.VisionSize
	for _, p := range m.Parts {
		n += p.Size
	}
	if m.Archive != nil {
		n = m.Archive.Size
	}
	return n
}

var catalog = []CatalogModel{
	{
		ID: "qwen3.5-0.8b", Name: "Qwen 3.5 Mini", Maker: "Alibaba", Params: "0.8B",
		Description: "Tiny and quick. Runs on almost any PC, even without a graphics card.",
		Repo:        "unsloth/Qwen3.5-0.8B-GGUF", File: "Qwen3.5-0.8B-Q4_K_M.gguf", Size: 532517120,
		Tags: []string{"Fastest", "Can think", "Sees images"}, License: "Apache 2.0",
		Vision: "mmproj-F16.gguf", VisionSize: 204987232,
	},
	{
		ID: "qwen3.5-2b", Name: "Qwen 3.5 Small", Maker: "Alibaba", Params: "2B",
		Description: "Small and fast. A good pick for laptops and older PCs.",
		Repo:        "unsloth/Qwen3.5-2B-GGUF", File: "Qwen3.5-2B-Q4_K_M.gguf", Size: 1280835840,
		Tags: []string{"Fast", "Can think", "Sees images"}, License: "Apache 2.0",
		Vision: "mmproj-F16.gguf", VisionSize: 668227264,
	},
	{
		ID: "qwen3.5-4b", Name: "Qwen 3.5", Maker: "Alibaba", Params: "4B",
		Description: "The best all-rounder for most PCs. Start here if you're not sure.",
		Repo:        "unsloth/Qwen3.5-4B-GGUF", File: "Qwen3.5-4B-Q4_K_M.gguf", Size: 2740937888,
		Tags: []string{"Recommended", "Can think", "Sees images"}, License: "Apache 2.0",
		Vision: "mmproj-F16.gguf", VisionSize: 672423616,
	},
	{
		ID: "gemma-4-e4b", Name: "Gemma 4", Maker: "Google", Params: "E4B",
		Description: "Google's efficient model. Strong at writing and explaining things clearly.",
		Repo:        "unsloth/gemma-4-E4B-it-GGUF", File: "gemma-4-E4B-it-Q4_K_M.gguf", Size: 4977171584,
		Tags: []string{"Writing", "Sees images"}, License: "Apache 2.0",
		Vision: "mmproj-F16.gguf", VisionSize: 990372672,
	},
	{
		ID: "qwen3.5-9b", Name: "Qwen 3.5 Plus", Maker: "Alibaba", Params: "9B",
		Description: "Noticeably smarter. Best with a graphics card that has 8 GB or more.",
		Repo:        "unsloth/Qwen3.5-9B-GGUF", File: "Qwen3.5-9B-Q4_K_M.gguf", Size: 5680522464,
		Tags: []string{"Smart", "Can think", "Sees images"}, License: "Apache 2.0",
		Vision: "mmproj-F16.gguf", VisionSize: 918166080,
	},
	{
		ID: "gemma-4-12b", Name: "Gemma 4 Large", Maker: "Google", Params: "12B",
		Description: "Thoughtful, well-written answers. Wants a graphics card with 10 GB or more.",
		Repo:        "unsloth/gemma-4-12b-it-GGUF", File: "gemma-4-12b-it-Q4_K_M.gguf", Size: 7121861440,
		Tags: []string{"Smart", "Writing", "Sees images"}, License: "Apache 2.0",
		Vision: "mmproj-F16.gguf", VisionSize: 175115840,
	},
	{
		ID: "gpt-oss-20b", Name: "gpt-oss", Maker: "OpenAI", Params: "20B",
		Description: "OpenAI's open model. Always reasons step by step before it answers.",
		Repo:        "unsloth/gpt-oss-20b-GGUF", File: "gpt-oss-20b-Q4_K_M.gguf", Size: 11624759488,
		Tags: []string{"Smart", "Always thinks"}, License: "Apache 2.0",
	},
	{
		ID: "qwen3-coder-30b", Name: "Qwen 3 Coder", Maker: "Alibaba", Params: "30B",
		Description: "Built for writing and fixing code. Quick for its size because only part of it runs at once.",
		Repo:        "unsloth/Qwen3-Coder-30B-A3B-Instruct-GGUF", File: "Qwen3-Coder-30B-A3B-Instruct-UD-Q4_K_XL.gguf", Size: 17665334432,
		Tags: []string{"Coding"}, License: "Apache 2.0",
	},
	{
		ID: "qwen3.8-27b", Name: "Qwen 3.8", Maker: "Alibaba", Params: "27B",
		Description: "Very capable for hard questions. Needs a high-end graphics card or lots of memory.",
		Repo:        "unsloth/Qwen3.8-27B-GGUF", File: "Qwen3.8-27B-UD-Q4_K_M.gguf", Size: 16464440224,
		Tags: []string{"Smartest", "Can think", "Sees images"}, License: "Apache 2.0",
		Vision: "mmproj-F16.gguf", VisionSize: 927607488,
	},
	{
		ID: "qwen3-coder-next", Name: "Qwen 3 Coder Next", Maker: "Alibaba", Params: "80B",
		Description: "The best open coding model that can run at home. Writes, fixes and explains code across whole projects. Needs about 40 GB of memory (graphics card plus system memory).",
		Repo:        "unsloth/Qwen3-Coder-Next-GGUF", File: "Qwen3-Coder-Next-UD-IQ4_XS.gguf", Size: 38429272064,
		Tags: []string{"Best for coding", "Big"}, License: "Apache 2.0",
	},

	// Pictures and video (stable-diffusion.cpp).
	{
		ID: "sdxl", Name: "Stable Diffusion XL", Maker: "Stability AI", Params: "3.5B",
		Description: "Makes pictures from a description: photos, art, logos, anything. Can also redo a picture you add.",
		Repo:        "stabilityai/stable-diffusion-xl-base-1.0", File: "sd_xl_base_1.0.safetensors", Size: 6938078334,
		Tags: []string{"Makes pictures", "Big"}, License: "OpenRAIL++",
		Kind: KindImage,
		Parts: []ModelPart{
			// The original decoder makes black pictures on graphics cards that
			// use 16-bit math; this fixed copy doesn't.
			{Role: "vae", Name: "Picture decoder for Stable Diffusion XL", Repo: "madebyollin/sdxl-vae-fp16-fix", File: "sdxl_vae.safetensors", Size: 334641162},
		},
	},
	{
		ID: "wan2.2-5b", Name: "Wan 2.2 Video", Maker: "Alibaba", Params: "5B",
		Description: "Makes short videos from a description, or brings a picture you add to life. Takes a few minutes per video.",
		Repo:        "QuantStack/Wan2.2-TI2V-5B-GGUF", File: "Wan2.2-TI2V-5B-Q4_K_M.gguf", Size: 3433116000,
		Tags: []string{"Makes videos", "Big"}, License: "Apache 2.0",
		Kind: KindVideo,
		Parts: []ModelPart{
			{Role: "text_encoder", Name: "Text reader for Wan 2.2", Repo: "city96/umt5-xxl-encoder-gguf", File: "umt5-xxl-encoder-Q4_K_M.gguf", Size: 3655145312},
			{Role: "vae", Name: "Video decoder for Wan 2.2", Repo: "Comfy-Org/Wan-2.2", File: "split_files/vae/wan2.2_vae.safetensors", Size: 1409400960},
		},
	},

	// Voice helpers. These are small and don't count toward the model limit.
	{
		ID: "nemotron-asr", Name: "Speech recognition", Maker: "NVIDIA", Params: "0.6B",
		Description: "Lets Phoenix understand what you say and what's in audio files. Works in 40+ languages.",
		Repo:        "nvidia/nemotron-3.5-asr-streaming-0.6b", File: "nemotron-3.5-asr-streaming-0.6b.q8_0.gguf", Size: 741548352,
		Revision: "1c8deaecc64b91f034d73e08dd8b64625eb3395d",
		Tags:     []string{"Understands speech"}, License: "NVIDIA Open Model License",
		Kind: KindListen,
	},
	{
		ID: "nemotron-diarization", Name: "Speaker detection", Maker: "NVIDIA", Params: "0.1B",
		Description: "Tells apart who is talking in audio files, up to 8 people, so transcripts say who said what.",
		Repo:        "nvidia/Nemotron-3-Diarization", File: "Nemotron-3-Diarization.q8_0.gguf", Size: 107012128,
		Revision: "f667ed73aee57d40cc39428eb768b4fd87a0a29e",
		Tags:     []string{"Who's talking"}, License: "NVIDIA Open Model License",
		Kind: KindSpeakers,
	},
	{
		ID: "kokoro", Name: "Kokoro voice", Maker: "hexgrad", Params: "82M",
		Description: "A natural-sounding voice, so Phoenix can read answers aloud and talk back to you.",
		Repo:        "hexgrad/Kokoro-82M", File: "kokoro-int8-multi-lang-v1_0/model.int8.onnx", Size: 132353410,
		Tags: []string{"Talks back"}, License: "Apache 2.0",
		Kind: KindVoice,
		Archive: &ArchiveSource{
			// The same Kokoro model converted to run without Python, by the
			// sherpa-onnx project.
			URL:    "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/kokoro-int8-multi-lang-v1_0.tar.bz2",
			Size:   132353410,
			Folder: "kokoro-int8-multi-lang-v1_0",
		},
	},
}

func findCatalog(id string) (CatalogModel, bool) {
	for _, m := range catalog {
		if m.ID == id {
			return m, true
		}
	}
	return CatalogModel{}, false
}

// catalogByRepo finds the recommended model that comes from a repository.
func catalogByRepo(repo string) (CatalogModel, bool) {
	for _, m := range catalog {
		if m.Repo == repo {
			return m, true
		}
	}
	return CatalogModel{}, false
}

func catalogByFile(repo, file string) (CatalogModel, bool) {
	for _, m := range catalog {
		if m.Repo == repo && m.File == file {
			return m, true
		}
	}
	return CatalogModel{}, false
}

// catalogByKind returns the first recommended model of a kind.
func catalogByKind(kind string) (CatalogModel, bool) {
	for _, m := range catalog {
		if m.kindOf() == kind {
			return m, true
		}
	}
	return CatalogModel{}, false
}

// partOf finds the recommended model that uses a file as one of its parts.
func partOf(repo, file string) (CatalogModel, ModelPart, bool) {
	for _, m := range catalog {
		for _, p := range m.Parts {
			if p.Repo == repo && p.File == file {
				return m, p, true
			}
		}
	}
	return CatalogModel{}, ModelPart{}, false
}

// fitForKind is fitFor, adjusted for what the model does. Speech and voice
// models run on the processor and fit anywhere; picture and video models
// can keep part of themselves in system memory while they work.
func fitForKind(kind string, size int64, sys *SystemInfo) Fit {
	switch kind {
	case KindListen, KindSpeakers, KindVoice:
		return Fit{Level: "great", Label: "Runs on your processor"}
	case KindImage, KindVideo:
		f := fitFor(size, sys)
		switch f.Level {
		case "great":
			f.Label = "Runs well on your graphics card"
		case "ok", "slow":
			f.Level, f.Label = "slow", "Works, but takes a while on this PC"
		}
		return f
	}
	return fitFor(size, sys)
}

// Fit says how well a model of a given size will run on this PC.
type Fit struct {
	Level string `json:"level"` // great, ok, slow, toobig, unknown
	Label string `json:"label"`
}

const gb = int64(1) << 30

// fitFor estimates whether a model file of `size` bytes runs well here.
// It needs room for the model plus about 1 GB for the conversation memory.
func fitFor(size int64, sys *SystemInfo) Fit {
	if sys == nil || sys.RAM == 0 {
		return Fit{Level: "unknown", Label: "Size " + humanBytes(size)}
	}
	need := size + gb
	vram := sys.BestVRAM
	// Leave about 3 GB of system memory for Windows and other apps.
	ram := sys.RAM - 3*gb
	if ram < 0 {
		ram = 0
	}
	switch {
	case vram >= need:
		return Fit{Level: "great", Label: "Runs great on your graphics card"}
	case vram+ram >= need && size <= 3*gb:
		return Fit{Level: "ok", Label: "Runs well on this PC"}
	case vram+ram >= need:
		return Fit{Level: "slow", Label: "Runs, but slowly on this PC"}
	default:
		return Fit{Level: "toobig", Label: "Too big for this PC"}
	}
}
