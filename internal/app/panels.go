package app

// panelViews maps a pending request kind to its view in frontend/src/views. Each panel kind adds itself in an init() of its own file.
var panelViews = map[string]string{}

func registerPanel(kind, view string) { panelViews[kind] = view }

// viewFor returns the view of kind, or "Generic" when no panel registered the kind.
func viewFor(kind string) string {
	if v, ok := panelViews[kind]; ok {
		return v
	}
	return "Generic"
}
