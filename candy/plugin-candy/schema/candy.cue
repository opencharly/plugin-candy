// plugin-candy's OWN self-contained CUE schema — the SINGLE SOURCE for this
// plugin's declaration surface. There is NO schema-less plugin: every plugin
// ships a non-empty, self-contained schema, served over Describe (the SDK splices
// `base ++ plugin` at the load gate), and this one DOCUMENTS the plugin's command
// surface.
//
// SELF-CONTAINED: it references NO base def, so it compiles STANDALONE — the exact
// property `cue exp gengotypes` needs to generate Go params, AND the property that
// lets the SDK compile it serve-side.
#CandyPlugin: {
	// The top-level command word this plugin serves.
	command: "candy"

	// The subcommands: `set` (dot-path mutation) + one `add-<fmt>` per
	// distro-format section, derived from sectionDistroPath so the verb set cannot
	// drift from the dispatch.
	subcommands: ["set", "add-rpm", "add-deb", "add-pac", "add-aur", "add-apk"]

	// What the plugin does, in one line (the public-docs surface).
	contract: string & !=""
}
