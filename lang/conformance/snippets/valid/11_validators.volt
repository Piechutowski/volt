// spec: §validators — a validators block names Go functions of the package over columns of the table
table users {
  wealth int
  debt int

  validators {
    Solvent(wealth, debt)
    DebtNonNegative(debt)
  }
}
