package db

// The real FADN survey partials — every da_* table injects one of these.
TablePartial da_base {
	idpk integer [pk]
	idgr text    [not null, default: '']
	rok  integer [not null, default: 0]
}

TablePartial da_head {
	idpk integer [pk]
	idgr text    [not null, default: '']
	rok  integer [not null, default: 0]
	Indexes { (rok, idgr) [unique] }
}

Table da_r_r {
	note: 'Ilości referencyjne'
	~da_head
	r_r_platsmg_tn text [not null, default: '', note: 'Czy w roku 2024 rolnik uczestniczył w systemie dla małych gospodarstw? [T/N], j.m.=kod, słownik=TakNie']
	r_r_platsmg_zl real [not null, default: 0,  note: 'Kwota płatności w ramach systemu dla małych gospodarstw, j.m.=zł']
	r_r_platbc_tn  text [not null, default: '', note: 'Czy przysługiwało prawo do płatności do powierzchni uprawy buraków cukrowych? [T/N], j.m.=kod, słownik=TakNie']
	r_r_platbc_ha  real [not null, default: 0,  note: 'Powierzchnia uprawy buraków cukrowych objęta płatnością, j.m.=ha']
	// … abridged: the full table has ~50 columns
}

Table da_i_u {
	note: 'Uprawy'
	~da_base
	i_u_kod text [not null, default: '', note: 'Kod, j.m.=kod, słownik=Kody']
	i_u_pow real [not null, default: 0,  note: 'Powierzchnia, j.m.=ha']
	i_u_zb  real [not null, default: 0,  note: 'Zbiór, j.m.=dt']
	Indexes { (rok, idgr, i_u_kod) [unique] }
	// … abridged
}

TableGroup DA {
	note: 'Survey data tables'
	da_r_r
	da_i_u
}

// nao v1 `Select` block (features.md): real SQL in the schema, a typed
// function out, prepare-validated against the generated DDL at gen time.
Select DaRRLatestRok {
	'SELECT MAX(rok) AS rok FROM da_r_r'
}
