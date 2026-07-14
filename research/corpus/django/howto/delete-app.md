# How to delete a Django application

Django provides the ability to group sets of features into Python packages called `applications</ref/applications/>`. When requirements change, apps may become obsolete or unnecessary. The following steps will help you delete an application safely.

1.  Remove all references to the app (imports, foreign keys etc.).
2.  Remove all models from the corresponding `models.py` file.
3.  Create relevant migrations by running `makemigrations`. This step generates a migration that deletes tables for the removed models, and any other required migration for updating relationships connected to those models.
4.  `Squash <migration-squashing>` out references to the app in other apps' migrations.
5.  Apply migrations locally, run tests, and verify the correctness of your project.
6.  Deploy/release your updated Django project.
7.  Remove the app from `INSTALLED_APPS`.
8.  Finally, remove the app's directory.
