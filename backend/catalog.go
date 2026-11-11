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
}

var catalog = []CatalogModel{
	{
		ID: "qwen3.5-0.8b", Name: "Qwen 3.5 Mini", Maker: "Alibaba", Params: "0.8B",
		Description: "Tiny and quick. Runs on almost any PC, even without a graphics card.",
		Repo:        "unsloth/Qwen3.5-0.8B-GGUF", File: "Qwen3.5-0.8B-Q4_K_M.gguf", Size: 532517120,
		Tags: []string{"Fastest", "Can think"}, License: "Apache 2.0",
	},
	{
		ID: "qwen3.5-2b", Name: "Qwen 3.5 Small", Maker: "Alibaba", Params: "2B",
		Description: "Small and fast. A good pick for laptops and older PCs.",
		Repo:        "unsloth/Qwen3.5-2B-GGUF", File: "Qwen3.5-2B-Q4_K_M.gguf", Size: 1280835840,
		Tags: []string{"Fast", "Can think"}, License: "Apache 2.0",
	},
	{
		ID: "qwen3.5-4b", Name: "Qwen 3.5", Maker: "Alibaba", Params: "4B",
		Description: "The best all-rounder for most PCs. Start here if you're not sure.",
		Repo:        "unsloth/Qwen3.5-4B-GGUF", File: "Qwen3.5-4B-Q4_K_M.gguf", Size: 2740937888,
		Tags: []string{"Recommended", "Can think"}, License: "Apache 2.0",
	},
	{
		ID: "gemma-4-e4b", Name: "Gemma 4", Maker: "Google", Params: "E4B",
		Description: "Google's efficient model. Strong at writing and explaining things clearly.",
		Repo:        "unsloth/gemma-4-E4B-it-GGUF", File: "gemma-4-E4B-it-Q4_K_M.gguf", Size: 4977171584,
		Tags: []string{"Writing"}, License: "Apache 2.0",
	},
	{
		ID: "qwen3.5-9b", Name: "Qwen 3.5 Plus", Maker: "Alibaba", Params: "9B",
		Description: "Noticeably smarter. Best with a graphics card that has 8 GB or more.",
		Repo:        "unsloth/Qwen3.5-9B-GGUF", File: "Qwen3.5-9B-Q4_K_M.gguf", Size: 5680522464,
		Tags: []string{"Smart", "Can think"}, License: "Apache 2.0",
	},
	{
		ID: "gemma-4-12b", Name: "Gemma 4 Large", Maker: "Google", Params: "12B",
		Description: "Thoughtful, well-written answers. Wants a graphics card with 10 GB or more.",
		Repo:        "unsloth/gemma-4-12b-it-GGUF", File: "gemma-4-12b-it-Q4_K_M.gguf", Size: 7121861440,
		Tags: []string{"Smart", "Writing"}, License: "Apache 2.0",
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
		Tags: []string{"Smartest", "Can think"}, License: "Apache 2.0",
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

func catalogByFile(repo, file string) (CatalogModel, bool) {
	for _, m := range catalog {
		if m.Repo == repo && m.File == file {
			return m, true
		}
	}
	return CatalogModel{}, false
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
