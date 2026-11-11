// mkres writes rsrc_windows_amd64.syso: the app icon, version details and
// a manifest (sharp text on high-DPI screens). Go embeds it automatically
// when building for Windows.
//
//	go run ./tools/mkres            (version from the VERSION file)
package main

import (
	"image"
	_ "image/png"
	"log"
	"os"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

func main() {
	v, err := os.ReadFile("VERSION")
	if err != nil {
		log.Fatal(err)
	}
	appVersion := strings.TrimSpace(string(v)) + ".0" // Windows wants four numbers

	f, err := os.Open("build/appicon.png")
	if err != nil {
		log.Fatal(err)
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}

	rs := winres.ResourceSet{}
	icon, err := winres.NewIconFromResizedImage(img, []int{256, 64, 48, 40, 32, 24, 20, 16})
	if err != nil {
		log.Fatal(err)
	}
	// ID 3 is where Wails looks for the window icon.
	if err := rs.SetIcon(winres.ID(3), icon); err != nil {
		log.Fatal(err)
	}

	rs.SetManifest(winres.AppManifest{
		Description:         "Phoenix AI",
		Compatibility:       winres.Win10AndAbove,
		ExecutionLevel:      winres.AsInvoker,
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
		LongPathAware:       true,
	})

	vi := version.Info{}
	vi.SetFileVersion(appVersion)
	vi.SetProductVersion(appVersion)
	vi.Set(version.LangDefault, version.ProductName, "Phoenix AI")
	vi.Set(version.LangDefault, version.FileDescription, "Phoenix AI")
	vi.Set(version.LangDefault, version.OriginalFilename, "PhoenixAI.exe")
	vi.Set(version.LangDefault, version.LegalCopyright, "Phoenix AI")
	rs.SetVersionInfo(vi)

	out, err := os.Create("rsrc_windows_amd64.syso")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if err := rs.WriteObject(out, winres.ArchAMD64); err != nil {
		log.Fatal(err)
	}
	log.Println("wrote rsrc_windows_amd64.syso")
}
