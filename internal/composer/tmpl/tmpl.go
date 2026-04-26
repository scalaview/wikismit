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

//go:embed event_flow_doc.tmpl
var EventFlowDocTmpl string

//go:embed callback_flow_doc.tmpl
var CallbackFlowDocTmpl string

var (
	VitepressConfigTmplParsed   template.Template
	VitepressMermaidTmplParsed  template.Template
	NavigationSectionTmplParsed template.Template
	EventFlowDocTmplParsed      template.Template
	CallbackFlowDocTmplParsed   template.Template
)

func init() {
	VitepressConfigTmplParsed = *template.Must(template.New("vitepress_config").Parse(VitepressConfigTmpl))
	VitepressMermaidTmplParsed = *template.Must(template.New("vitepress_mermaid").Parse(VitepressMermaidTmpl))
	NavigationSectionTmplParsed = *template.Must(template.New("navigation_section").Parse(NavigationSectionTmpl))
	EventFlowDocTmplParsed = *template.Must(template.New("event_flow_doc").Parse(EventFlowDocTmpl))
	CallbackFlowDocTmplParsed = *template.Must(template.New("callback_flow_doc").Parse(CallbackFlowDocTmpl))
}
