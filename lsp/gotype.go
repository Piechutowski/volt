package lsp

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lsp/protocol"
)

func (d *Document) goTypeDefinition(offset int) *protocol.Location {
	if d.Info == nil {
		return nil
	}
	var td *check.TypeInfo
	for _, t := range d.Info.Types {
		if n := t.Decl.Name; n != nil && offset >= int(n.Pos().Offset()) && offset <= int(n.End().Offset()) {
			td = t
		}
	}
	if td == nil {
		return nil
	}
	path := pathFromURI(d.URI)
	dir := filepath.Dir(path)
	if td.Path != "" {
		if dir = goPackageDir(path, td.Path); dir == "" {
			return nil
		}
	}
	_, types, _ := lang.GoFuncsOf(dir, lang.GoSourcesRead(disk.Source{}, dir))
	gt, ok := types[lastPart(td.Decl.Name).Name()]
	if !ok {
		return nil
	}
	text, err := os.ReadFile(gt.File)
	if err != nil {
		return nil
	}
	return &protocol.Location{
		URI: "file://" + gt.File,
		Range: protocol.Range{
			Start: offsetToLSP(string(text), int(gt.Pos.Offset())),
			End:   offsetToLSP(string(text), int(gt.End.Offset())),
		},
	}
}

func goPackageDir(volt, importPath string) string {
	root, ok := projectRootOf(volt)
	if !ok {
		root = filepath.Dir(volt)
	}
	cmd := exec.Command("go", "list", "-find", "-buildvcs=false", "-f", "{{.Dir}}", "--", importPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(bytes.TrimSpace(out)))
}
