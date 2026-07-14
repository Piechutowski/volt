# How to deploy Django

Django is full of shortcuts to make web developers' lives easier, but all those tools are of no use if you can't easily deploy your sites. Since Django's inception, ease of deployment has been a major goal.

There are many options for deploying your Django application, based on your architecture or your particular business needs, but that discussion is outside the scope of what Django can give you as guidance.

Django, being a web framework, needs a web server in order to operate. And since most web servers don't natively speak Python, we need an interface to make that communication happen. The `runserver` command starts a lightweight development server, which is not suitable for production.

Django currently supports two interfaces: WSGI and ASGI.

- [WSGI](https://wsgi.readthedocs.io/en/latest/) is the main Python standard for communicating between web servers and applications, but it only supports synchronous code.
- [ASGI](https://asgi.readthedocs.io/en/latest/) is the new, asynchronous-friendly standard that will allow your Django site to use asynchronous Python features, and asynchronous Django features as they are developed.

You should also consider how you will handle `static files
</howto/static-files/deployment>` for your application, and how to handle `error reporting</howto/error-reporting>`.

Finally, before you deploy your application to production, you should run through our `deployment checklist </howto/deployment/checklist>` to ensure that your configurations are suitable.

<div class="toctree" maxdepth="2">

wsgi/index asgi/index checklist

</div>
