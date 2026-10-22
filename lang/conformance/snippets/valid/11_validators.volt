// spec: §validators — a validators block names Go functions of the package over columns of the table
Table users {
  wealth int
  debt int

  Validators {
    Solvent(wealth, debt)
    DebtNonNegative(debt)
  }
}
