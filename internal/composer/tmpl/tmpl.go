package tmpl

import (
	_ "embed"
	"text/template"
)

//go:embed vitepressconfig.tmpl
var VitepressConfigTmpl string

//go:embed vitepressmermaid.tmpl
var VitepressMermaidTmpl string

var (
	VitepressConfigTmplParsed  template.Template
	VitepressMermaidTmplParsed template.Template
)

func init() {
	VitepressConfigTmplParsed = *template.Must(template.New("vitepress_config").Parse(VitepressConfigTmpl))
	VitepressMermaidTmplParsed = *template.Must(template.New("vitepress_mermaid").Parse(VitepressMermaidTmpl))
}
