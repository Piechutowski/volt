// analyzers: emptybody

table t {
  id int
  indexes { //WANT emptybody
  }
  validators { //WANT emptybody
  }
}
