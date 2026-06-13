package renderer

import "sort"

// param is a FreeSWITCH <param name=".." value=".."/> / <variable .../> element.
type param struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// sortedParams converts a map into a deterministically ordered slice of params,
// sorted by key so the rendered XML is stable for tests and diffs.
func sortedParams(m map[string]string) []param {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]param, 0, len(keys))
	for _, k := range keys {
		out = append(out, param{Name: k, Value: m[k]})
	}
	return out
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

// notFoundDocument is returned to mod_xml_curl when nothing matches, so that
// FreeSWITCH falls back to its on-disk configuration.
const NotFoundDocument = xmlHeader + `<document type="freeswitch/xml">
  <section name="result">
    <result status="not found"/>
  </section>
</document>
`
