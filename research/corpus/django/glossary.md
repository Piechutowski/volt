# Glossary

<div class="glossary">

concrete model  
A non-abstract (`abstract=False
<django.db.models.Options.abstract>`) model.

field  
An attribute on a `model`; a given field usually maps directly to a single database column.

See `/topics/db/models`.

generic view  
A higher-order `view` function that provides an abstract/generic implementation of a common idiom or pattern found in view development.

See `/topics/class-based-views/index`.

model  
Models store your application's data.

See `/topics/db/models`.

MTV  
"Model-template-view"; a software pattern, similar in style to MVC, but a better description of the way Django does things.

See `the FAQ entry <faq-mtv>`.

MVC  
<span class="title-ref">Model-view-controller</span>\_\_; a software pattern. Django `follows MVC
to some extent <faq-mtv>`.

\_\_ <https://en.wikipedia.org/wiki/Model-view-controller>

project  
A Python package -- i.e. a directory of code -- that contains all the settings for an instance of Django. This would include database configuration, Django-specific options and application-specific settings.

property  
Also known as "managed attributes", and a feature of Python since version 2.2. This is a neat way to implement attributes whose usage resembles attribute access, but whose implementation uses method calls.

See `property`.

queryset  
An object representing some set of rows to be fetched from the database.

See `/topics/db/queries`.

slug  
A short label for something, containing only letters, numbers, underscores or hyphens. They're generally used in URLs. For example, in a typical blog entry URL:

<div class="parsed-literal">

<https://www.djangoproject.com/weblog/2008/apr/12/>**spring**/

</div>

the last bit (`spring`) is the slug.

template  
A chunk of text that acts as formatting for representing data. A template helps to abstract the presentation of data from the data itself.

See `/topics/templates`.

view  
A function responsible for rendering a page.

</div>
