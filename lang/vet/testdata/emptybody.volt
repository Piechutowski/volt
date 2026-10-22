// analyzers: emptybody
Project p { //WANT emptybody
}
Table t {
  id int
  Indexes { //WANT emptybody
  }
  Validators { //WANT emptybody
  }
}
