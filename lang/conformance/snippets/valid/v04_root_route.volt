// spec: §routes — the root full path registers as /{$}
package app

scope / {
	get / Home.Index
}
