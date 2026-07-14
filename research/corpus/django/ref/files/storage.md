# File storage API

<div class="module">

django.core.files.storage

</div>

## Getting the default storage class

Django provides convenient ways to access the default storage class:

<div class="data">

storages

A dictionary-like object that allows retrieving a storage instance using its alias as defined by `STORAGES`.

`storages` has an attribute `backends`, which defaults to the raw value provided in `STORAGES`.

Additionally, `storages` provides a `create_storage()` method that accepts the dictionary used in `STORAGES` for a backend, and returns a storage instance based on that backend definition. This may be useful for third-party packages needing to instantiate storages in tests:

``` pycon
>>> from django.core.files.storage import storages
>>> storages.backends
{'default': {'BACKEND': 'django.core.files.storage.FileSystemStorage'},
 'staticfiles': {'BACKEND': 'django.contrib.staticfiles.storage.StaticFilesStorage'},
 'custom': {'BACKEND': 'package.storage.CustomStorage'}}
>>> storage_instance = storages.create_storage({"BACKEND": "package.storage.CustomStorage"})
```

</div>

<div class="DefaultStorage">

`~django.core.files.storage.DefaultStorage` provides lazy access to the default storage system as defined by `default` key in `STORAGES`. `DefaultStorage` uses `~django.core.files.storage.storages` internally.

</div>

<div class="data">

default_storage

`~django.core.files.storage.default_storage` is an instance of the `~django.core.files.storage.DefaultStorage`.

</div>

## The `FileSystemStorage` class

<div class="FileSystemStorage(location=None, base_url=None, file_permissions_mode=None, directory_permissions_mode=None, allow_overwrite=False)">

The `~django.core.files.storage.FileSystemStorage` class implements basic file storage on a local filesystem. It inherits from `~django.core.files.storage.Storage` and provides implementations for all the public methods thereof.

<div class="note">

<div class="title">

Note

</div>

The `FileSystemStorage.delete()` method will not raise an exception if the given file name does not exist.

</div>

<div class="attribute">

location

Absolute path to the directory that will hold the files. Defaults to the value of your `MEDIA_ROOT` setting.

</div>

<div class="attribute">

base_url

URL that serves the files stored at this location. Defaults to the value of your `MEDIA_URL` setting.

</div>

<div class="attribute">

file_permissions_mode

The file system permissions that the file will receive when it is saved. Defaults to `FILE_UPLOAD_PERMISSIONS`.

</div>

<div class="attribute">

directory_permissions_mode

The file system permissions that the directory will receive when it is saved. Defaults to `FILE_UPLOAD_DIRECTORY_PERMISSIONS`.

</div>

<div class="attribute">

allow_overwrite

Flag to control allowing saving a new file over an existing one. Defaults to `False`.

</div>

<div class="method">

get_created_time(name)

Returns a `~datetime.datetime` of the system's ctime, i.e. `os.path.getctime`. On some systems (like Unix), this is the time of the last metadata change, and on others (like Windows), it's the creation time of the file.

</div>

</div>

## The `InMemoryStorage` class

<div class="InMemoryStorage(location=None, base_url=None, file_permissions_mode=None, directory_permissions_mode=None)">

The `~django.core.files.storage.InMemoryStorage` class implements a memory-based file storage. It has no persistence, but can be useful for speeding up tests by avoiding disk access.

<div class="attribute">

location

Absolute path to the directory name assigned to files. Defaults to the value of your `MEDIA_ROOT` setting.

</div>

<div class="attribute">

base_url

URL that serves the files stored at this location. Defaults to the value of your `MEDIA_URL` setting.

</div>

<div class="attribute">

file_permissions_mode

The file system permissions assigned to files, provided for compatibility with `FileSystemStorage`. Defaults to `FILE_UPLOAD_PERMISSIONS`.

</div>

<div class="attribute">

directory_permissions_mode

The file system permissions assigned to directories, provided for compatibility with `FileSystemStorage`. Defaults to `FILE_UPLOAD_DIRECTORY_PERMISSIONS`.

</div>

</div>

## The `Storage` class

<div class="Storage">

The `~django.core.files.storage.Storage` class provides a standardized API for storing files, along with a set of default behaviors that all other storage systems can inherit or override as necessary.

<div class="note">

<div class="title">

Note

</div>

When methods return naive `datetime` objects, the effective timezone used will be the current value of `os.environ['TZ']`; note that this is usually set from Django's `TIME_ZONE`.

</div>

<div class="method">

delete(name)

Deletes the file referenced by `name`. If deletion is not supported on the target storage system this will raise `NotImplementedError` instead.

</div>

<div class="method">

exists(name)

Returns `True` if a file referenced by the given name already exists in the storage system.

</div>

<div class="method">

get_accessed_time(name)

Returns a `~datetime.datetime` of the last accessed time of the file. For storage systems unable to return the last accessed time this will raise `NotImplementedError`.

If `USE_TZ` is `True`, returns an aware `datetime`, otherwise returns a naive `datetime` in the local timezone.

</div>

<div class="method">

get_alternative_name(file_root, file_ext)

Returns an alternative filename based on the `file_root` and `file_ext` parameters, an underscore plus a random 7 character alphanumeric string is appended to the filename before the extension.

</div>

<div class="method">

get_available_name(name, max_length=None)

Returns a filename based on the `name` parameter that's free and available for new content to be written to on the target storage system.

The length of the filename will not exceed `max_length`, if provided. If a free unique filename cannot be found, a `SuspiciousFileOperation
<django.core.exceptions.SuspiciousOperation>` exception will be raised.

If a file with `name` already exists, `get_alternative_name` is called to obtain an alternative name.

</div>

<div class="method">

get_created_time(name)

Returns a `~datetime.datetime` of the creation time of the file. For storage systems unable to return the creation time this will raise `NotImplementedError`.

If `USE_TZ` is `True`, returns an aware `datetime`, otherwise returns a naive `datetime` in the local timezone.

</div>

<div class="method">

get_modified_time(name)

Returns a `~datetime.datetime` of the last modified time of the file. For storage systems unable to return the last modified time this will raise `NotImplementedError`.

If `USE_TZ` is `True`, returns an aware `datetime`, otherwise returns a naive `datetime` in the local timezone.

</div>

<div class="method">

get_valid_name(name)

Returns a filename based on the `name` parameter that's suitable for use on the target storage system.

</div>

<div class="method">

generate_filename(filename)

Validates the `filename` by calling `get_valid_name` and returns a filename to be passed to the `save` method.

The `filename` argument may include a path as returned by `FileField.upload_to <django.db.models.FileField.upload_to>`. In that case, the path won't be passed to `get_valid_name` but will be prepended back to the resulting name.

The default implementation uses `os.path` operations. Override this method if that's not appropriate for your storage.

</div>

<div class="method">

listdir(path)

Lists the contents of the specified path, returning a 2-tuple of lists; the first item being directories, the second item being files. For storage systems that aren't able to provide such a listing, this will raise a `NotImplementedError` instead.

</div>

<div class="method">

open(name, mode='rb')

Opens the file given by `name`. Note that although the returned file is guaranteed to be a `File` object, it might actually be some subclass. In the case of remote file storage this means that reading/writing could be quite slow, so be warned.

</div>

<div class="method">

path(name)

The local filesystem path where the file can be opened using Python's standard `open()`. For storage systems that aren't accessible from the local filesystem, this will raise `NotImplementedError` instead.

</div>

<div class="method">

save(name, content, max_length=None)

Saves a new file using the storage system, preferably with the name specified. If there already exists a file with this name `name`, the storage system may modify the filename as necessary to get a unique name. The actual name of the stored file will be returned.

The `max_length` argument is passed along to `get_available_name`.

The `content` argument must be an instance of `django.core.files.File` or a file-like object that can be wrapped in `File`.

</div>

<div class="method">

size(name)

Returns the total size, in bytes, of the file referenced by `name`. For storage systems that aren't able to return the file size this will raise `NotImplementedError` instead.

</div>

<div class="method">

url(name)

Returns the URL where the contents of the file referenced by `name` can be accessed. For storage systems that don't support access by URL this will raise `NotImplementedError` instead.

</div>

</div>

<div class="admonition">

There are community-maintained solutions too!

Django has a vibrant ecosystem. There are storage backends highlighted on the [Community Ecosystem](https://www.djangoproject.com/community/ecosystem/#storage-and-static-files) page. The Django Packages [Storage Backends grid](https://djangopackages.org/grids/g/storage-backends/) has even more options for you!

</div>
