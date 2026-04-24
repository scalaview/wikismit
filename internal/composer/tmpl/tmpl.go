package tmpl

import (
	_ "embed"
	"text/template"
)

//go:embed vitepressconfig.tmpl
var VitepressConfigTmpl string

//go:embed vitepressmermaid.tmpl
var VitepressMermaidTmpl string

//go:embed navigation_section.tmpl
var NavigationSectionTmpl string

var (
	VitepressConfigTmplParsed   template.Template
	VitepressMermaidTmplParsed  template.Template
	NavigationSectionTmplParsed template.Template
)

func init() {
	VitepressConfigTmplParsed = *template.Must(template.New("vitepress_config").Parse(VitepressConfigTmpl))
	VitepressMermaidTmplParsed = *template.Must(template.New("vitepress_mermaid").Parse(VitepressMermaidTmpl))
	NavigationSectionTmplParsed = *template.Must(template.New("navigation_section").Parse(NavigationSectionTmpl))
}
