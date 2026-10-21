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
