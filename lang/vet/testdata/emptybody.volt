// analyzers: emptybody
project p { //WANT emptybody
}
table t {
  id int
  indexes { //WANT emptybody
  }
  validators { //WANT emptybody
  }
}
