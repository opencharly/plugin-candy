// plugin-candy's OWN self-contained CUE schema — the SINGLE SOURCE for this plugin's
// served declaration surface (there is no schema-less plugin: every plugin ships a
// non-empty schema over Describe).
//
// SELF-CONTAINED and PACKAGE-LESS: it references no base def and carries no package
// clause, so it compiles STANDALONE — the property the SDK's serve-side compile needs
// and the property that lets the host splice `base ++ plugin` at the load gate
// (registerPluginUnitSchema); a self-contained schema that will not splice is a LOUD
// load failure.
//
// NO GO CONSUMER: the plugin declares no typed `plugin_input` (its authored input is
// its pass-through CLI grammar), so this schema generates NO `params` package and has
// NO `cue exp gengotypes` artifact — it is the SERVED documentation/config surface,
// not a code-generation source.
//
// It DOCUMENTS the `command:candy` surface: the `set` word plus the `add-<fmt>` words the dispatch serves (a manual mirror of the dispatch's distro-format list).
#CandyPlugin: {
	// The top-level command word this plugin serves.
	command: "candy"

	// The subcommands: `set` (dot-path mutation) + one `add-<fmt>` per
	// distro-format section. This is a MANUAL mirror of the dispatch's
	// distro-format list (`sectionDistroPath`); CUE cannot derive it at build time.
	subcommands: ["set", "add-rpm", "add-deb", "add-pac", "add-aur", "add-apk"]

	// What the plugin does, in one line (the public-docs surface).
	contract: string & !=""
}
