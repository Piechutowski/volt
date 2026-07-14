# The `File` object

The `django.core.files` module and its submodules contain built-in classes for basic file handling in Django.

<div class="currentmodule">

django.core.files

</div>

## The `File` class

<div class="File(file_object, name=None)">

The `File` class is a thin wrapper around a Python `file object` with some Django-specific additions. Internally, Django uses this class when it needs to represent a file.

`File` objects have the following attributes and methods:

<div class="attribute">

name

The name of the file including the relative path from `MEDIA_ROOT`.

</div>

<div class="attribute">

size

The size of the file in bytes.

</div>

<div class="attribute">

file

The underlying `file object` that this class wraps.

<div class="admonition">

Be careful with this attribute in subclasses.

Some subclasses of `File`, including `~django.core.files.base.ContentFile` and `~django.db.models.fields.files.FieldFile`, may replace this attribute with an object other than a Python `file
object`. In these cases, this attribute may itself be a `File` subclass (and not necessarily the same subclass). Whenever possible, use the attributes and methods of the subclass itself rather than the those of the subclass's `file` attribute.

</div>

</div>

<div class="attribute">

mode

The read/write mode for the file.

</div>

<div class="method">

open(mode=None, *args,*\*kwargs)

Open or reopen the file (which also does `File.seek(0)`). The `mode` argument allows the same values as Python's built-in `python:open`. `*args` and `**kwargs` are passed after `mode` to Python's built-in `python:open`.

When reopening a file, `mode` will override whatever mode the file was originally opened with; `None` means to reopen with the original mode.

It can be used as a context manager, e.g. `with file.open() as f:`.

</div>

<div class="method">

\_\_iter\_\_()

Iterate over the file yielding one line at a time.

</div>

<div class="method">

chunks(chunk_size=None)

Iterate over the file yielding "chunks" of a given size. `chunk_size` defaults to 64 KB.

This is especially useful with very large files since it allows them to be streamed off disk and avoids storing the whole file in memory.

</div>

<div class="method">

multiple_chunks(chunk_size=None)

Returns `True` if the file is large enough to require multiple chunks to access all of its content give some `chunk_size`.

</div>

<div class="method">

close()

Close the file.

</div>

In addition to the listed methods, `~django.core.files.File` exposes the following attributes and methods of its `file` object: `encoding`, `fileno`, `flush`, `isatty`, `newlines`, `read`, `readinto`, `readline`, `readlines`, `seek`, `tell`, `truncate`, `write`, `writelines`, `readable()`, `writable()`, and `seekable()`.

</div>

<div class="currentmodule">

django.core.files.base

</div>

## The `ContentFile` class

<div class="ContentFile(content, name=None)">

The `ContentFile` class inherits from `~django.core.files.File`, but unlike `~django.core.files.File` it operates on string content (bytes also supported), rather than an actual file. For example:

    from django.core.files.base import ContentFile

    f1 = ContentFile("esta frase está en español")
    f2 = ContentFile(b"these are bytes")

</div>

<div class="currentmodule">

django.core.files.images

</div>

## The `ImageFile` class

<div class="ImageFile(file_object, name=None)">

Django provides a built-in class specifically for images. `django.core.files.images.ImageFile` inherits all the attributes and methods of `~django.core.files.File`, and additionally provides the following:

<div class="attribute">

width

Width of the image in pixels.

</div>

<div class="attribute">

height

Height of the image in pixels.

</div>

</div>

<div class="currentmodule">

django.core.files

</div>

## Additional methods on files attached to objects

Any `File` that is associated with an object (as with `Car.photo`, below) will also have a couple of extra methods:

<div class="method">

File.save(name, content, save=True)

Saves a new file with the file name and contents provided. This will not replace the existing file, but will create a new file and update the object to point to it. If `save` is `True`, the model's `save()` method will be called once the file is saved. That is, these two lines:

``` pycon
>>> car.photo.save("myphoto.jpg", content, save=False)
>>> car.save()
```

are equivalent to:

``` pycon
>>> car.photo.save("myphoto.jpg", content, save=True)
```

Note that the `content` argument must be an instance of either `File` or of a subclass of `File`, such as `~django.core.files.base.ContentFile`.

</div>

<div class="method">

File.delete(save=True)

Removes the file from the model instance and deletes the underlying file. If `save` is `True`, the model's `save()` method will be called once the file is deleted.

</div>
