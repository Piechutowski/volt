package db

// Every ms_* series table injects one of these shared headers.
TablePartial ms_base {
	id     integer [pk]
	region text    [not null, default: '']
	year   integer [not null, default: 0]
}

TablePartial ms_head {
	id     integer [pk]
	region text    [not null, default: '']
	year   integer [not null, default: 0]
	Indexes { (year, region) [unique] }
}

Table ms_revenue {
	note: 'Revenue by region and year'
	~ms_head
	rev_recurring_flag text [not null, default: '', note: 'Is the plan billed on a recurring schedule? [Y/N], unit=code, dict=YesNo']
	rev_recurring_amt  real [not null, default: 0,  note: 'Revenue from recurring plans, unit=EUR']
	rev_discount_flag  text [not null, default: '', note: 'Was a volume discount applied? [Y/N], unit=code, dict=YesNo']
	rev_discount_pct   real [not null, default: 0,  note: 'Share of revenue discounted, unit=%']
	// … abridged: the full table has ~50 columns
}

Table ms_usage {
	note: 'Feature usage'
	~ms_base
	use_code  text [not null, default: '', note: 'Feature code, unit=code, dict=Features']
	use_seats real [not null, default: 0,  note: 'Licensed seats, unit=seats']
	use_calls real [not null, default: 0,  note: 'API calls, unit=thousands']
	Indexes { (year, region, use_code) [unique] }
	// … abridged
}

TableGroup MS {
	note: 'Measurement series tables'
	ms_revenue
	ms_usage
}

// nao v1 `Select` block (features.md): real SQL in the schema, a typed
// function out, prepare-validated against the generated DDL at gen time.
Select MsRevenueLatestYear {
	'SELECT MAX(year) AS year FROM ms_revenue'
}
