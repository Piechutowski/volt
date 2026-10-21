// spec: §migrations, a migration may stand in any file of its package, alone
package db

Table users {
  id int [pk, increment]
}
