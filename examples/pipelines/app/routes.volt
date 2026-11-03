package app

pipeline api {
	use volt.RequestID
	use Stamp
}

pipeline vault {
	use RequireKey
}

scope / [pipe: api] {
	get /open Pages.Open [name: open]

	scope /vault [pipe: vault, name: vault] {
		get /secret Vault.Secret
	}
}
