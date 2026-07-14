# How to use Django with Uvicorn

[Uvicorn](https://www.uvicorn.org/) is an ASGI server based on `uvloop` and `httptools`, with an emphasis on speed.

## Installing Uvicorn

You can install Uvicorn with `pip`:

``` shell
python -m pip install uvicorn
```

## Running Django in Uvicorn

When Uvicorn is installed, a `uvicorn` command is available which runs ASGI applications. Uvicorn needs to be called with the location of a module containing an ASGI application object, followed by what the application is called (separated by a colon).

For a typical Django project, invoking Uvicorn would look like:

``` shell
python -m uvicorn myproject.asgi:application
```

This will start one process listening on `127.0.0.1:8000`. It requires that your project be on the Python path; to ensure that run this command from the same directory as your `manage.py` file.

In development mode, you can add `--reload` to cause the server to reload any time a file is changed on disk.

For more advanced usage, please read the [Uvicorn documentation](https://www.uvicorn.org/).

## Deploying Django using Uvicorn and Gunicorn

[Gunicorn](https://gunicorn.org/) is a robust web server that implements process monitoring and automatic restarts. This can be useful when running Uvicorn in a production environment.

To install Uvicorn and Gunicorn, use the following:

``` shell
python -m pip install uvicorn uvicorn-worker gunicorn
```

Then start Gunicorn using the Uvicorn worker class like this:

``` shell
python -m gunicorn myproject.asgi:application -k uvicorn_worker.UvicornWorker
```
