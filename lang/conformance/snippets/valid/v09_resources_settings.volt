// spec: §V5.3
package app

Scope / {
	resources users  [api, param: uid]
	resources posts  [only: (index, show)]
	resources drafts [except: (delete)]
}
