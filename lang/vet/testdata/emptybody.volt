// analyzers: emptybody
Project p { //WANT emptybody
}
Table t {
  id int
  indexes { //WANT emptybody
  }
  validators { //WANT emptybody
  }
}
TableGroup g { //WANT emptybody
}
DiagramView v {
  Tables { //WANT emptybody
  }
  Notes { * }
}
