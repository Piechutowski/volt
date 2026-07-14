# Sending email

<div class="module" synopsis="Helpers to send email.">

django.core.mail

</div>

Django provides wrappers for Python's `email` and `smtplib` modules to simplify composing and sending email. Django's email framework also supports swapping in different delivery mechanisms: you can direct email to the console or a file during development and an SMTP server or email service provider in production.

The code lives in the `django.core.mail` module.

## Quick examples

Use `send_mail` for straightforward email sending. For example, to send a plain text message:

    from django.core.mail import send_mail

    send_mail(
        "Subject here",
        "Here is the message.",
        "from@example.com",
        ["to@example.com"],
    )

When additional email sending functionality is needed, use the `EmailMessage <topic-email-message>` or `EmailMultiAlternatives` class. For example, to send a multipart email that includes both HTML and plain text versions with a specific template and custom headers, you can use the following approach:

    from django.core.mail import EmailMultiAlternatives
    from django.template.loader import render_to_string

    # First, render the plain text content.
    text_content = render_to_string(
        "templates/emails/my_email.txt",
        context={"my_variable": 42},
    )

    # Secondly, render the HTML content.
    html_content = render_to_string(
        "templates/emails/my_email.html",
        context={"my_variable": 42},
    )

    # Then, create a multipart email instance.
    msg = EmailMultiAlternatives(
        subject="Subject here",
        body=text_content,
        from_email="from@example.com",
        to=["to@example.com"],
        headers={"List-Unsubscribe": "<mailto:unsub@example.com>"},
    )

    # Lastly, attach the HTML content to the email instance and send.
    msg.attach_alternative(html_content, "text/html")
    msg.send()

## Configuring email

New Django projects are not configured to send email by default. Instead, email is printed to the console as a development aid (for projects created with `startproject`) or results in a `MailerDoesNotExist` error (when the `MAILERS` setting isn't defined).

Use the `MAILERS` setting to tell Django how to send email. For example, to send through an SMTP server running on the local machine:

    MAILERS = {
        "default": {
            "BACKEND": "django.core.mail.backends.smtp.EmailBackend",
            "OPTIONS": {
                "host": "localhost",
            },
        },
    }

Django abstracts the email sending process into an "email backend" class. `topic-email-backends` lists the email backends that come with Django.

The example above uses Django's `SMTP email backend
<topic-email-smtp-backend>`, which sends using the standard [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol) protocol. This backend is useful for many production configurations, including SMTP servers in your own infrastructure and most commercial email service providers (ESPs). There are also `third-party email backends
<topic-third-party-email-backends>` available that integrate directly with ESP APIs or add other sending features.

During development or testing you often don't want to send email at all. Django's test runner `automatically overrides <topics-testing-email>` the `MAILERS` configuration to substitute Django's `memory email
backend <topic-email-memory-backend>`. This prevents test cases from sending real email and gives them access to the messages that would have been sent. `topics/email:Configuring email for development` discusses some other approaches.

<div class="versionchanged">

6.1

In earlier releases, Django defaulted to sending email through an SMTP server running on localhost, using the now-deprecated `EMAIL_BACKEND` and related settings.

</div>

<div class="deprecated">

6.1

Until Django 7.0, if the `MAILERS` setting is not defined then the earlier behavior still applies: Django will default to using an SMTP server on localhost (but will issue deprecation warnings). Starting in Django 7.0, attempts to send email without `MAILERS` defined will result in a `MailerDoesNotExist` error.

Existing projects can opt into the new behavior early by adding `MAILERS` to `settings.py`. See `migrating-to-mailers`.

</div>

### Multiple mailers

Sometimes different types of email need to be sent in different ways: e.g., internal vs. external email, different SMTP servers for users in different regions, using different services for transactional notifications and bulk marketing email, etc.

The `MAILERS` setting can define multiple mail configurations. For example:

    import os

    MAILERS = {
        "default": {
            "BACKEND": "django.core.mail.backends.smtp.EmailBackend",
            "OPTIONS": {
                "host": "smtp.example.net",
                "use_tls": True,
                "username": os.environ["EMAIL_ACCOUNT_ID"],
                "password": os.environ["EMAIL_API_KEY"],
            },
        },
        "notifications": {
            "BACKEND": "example.third.party.EmailBackend",
            "OPTIONS": {
                "api_key": os.environ["THIRD_PARTY_API_KEY"],
                "region": "eu",
            },
        },
        "admin": {
            "BACKEND": "django.core.mail.backends.smtp.EmailBackend",
            "OPTIONS": {
                "host": "localhost",
            },
        },
    }

This defines three mailer configurations:

- `"default"` sends through an SMTP server at `smtp.example.net` with a TLS secured connection. It reads an account id and API key from environment variables and uses them as the SMTP authentication username and password. (Many SMTP services use some variation of this authentication scheme.)
- `"notifications"` sends through a hypothetical commercial email service, using a third-party EmailBackend that connects directly to their API. (See `topic-third-party-email-backends` for pointers on locating real, community maintained email backend packages.)
- `"admin"` sends through an SMTP server running on `localhost`, with no other options required.

With this configuration, you can provide the `using` argument to Django's `email sending functions <topic-email-sending>` to specify a particular mailer configuration:

    from django.core.mail import send_mail

    send_mail(
        "Account activated",
        "Congratulations, you're all ready to use our Django app!",
        "from@example.com",
        ["user@example.com"],
        using="notifications",
    )

If `using` is not specified, Django uses the mailer defined for the `"default"` configuration.

With reusable apps or Django features that send email for you, there may be an option to use a specific mailer configuration. For example, Django's logging `~django.utils.log.AdminEmailHandler` allows specifying the mailer configuration in its `using` option.

## Sending messages

`!django.core.mail` provides functions for conveniently sending email, as well as classes for building and sending more complex email messages with attachments and multiple content types.

<div class="note">

<div class="title">

Note

</div>

The character set of email sent with `django.core.mail` will be set to the value of your `DEFAULT_CHARSET` setting.

</div>

### `send_mail()`

<div class="function">

send_mail(subject, message, from_email, recipient_list, \*, fail_silently=False, auth_user=None, auth_password=None, connection=None, html_message=None)

</div>

`django.core.mail.send_mail()` sends a single email message.

The `subject`, `message`, `from_email` and `recipient_list` parameters are required.

- `subject`: A string.
- `message`: A string.
- `from_email`: A string. If `None`, Django will use the value of the `DEFAULT_FROM_EMAIL` setting.
- `recipient_list`: A list of strings, each an email address. Each member of `recipient_list` will see the other recipients in the "To:" field of the email message.

The following parameters are optional, and must be given as keyword arguments if used.

- `fail_silently`: A boolean, default `False`. If set `True`, `send_mail()` will suppress some errors during sending. (The exact exceptions ignored depend on the email backend in use.)
- `auth_user`: The optional username to use to authenticate to the SMTP server. If this isn't provided, Django will use the value of the `EMAIL_HOST_USER` setting.
- `auth_password`: The optional password to use to authenticate to the SMTP server. If this isn't provided, Django will use the value of the `EMAIL_HOST_PASSWORD` setting.
- `connection`: The optional email backend to use to send the mail. If unspecified, an instance of the default backend will be used. See the documentation on `Email backends <topic-email-backends>` for more details.
- `html_message`: If `html_message` is provided, the resulting email will be a `multipart/alternative` email with `message` as the `text/plain` content type and `html_message` as the `text/html` content type.
- `using`: An optional `MAILERS` alias to use to send the mail. If unspecified, the default mailer configuration will be used.

`fail_silently`, `auth_user`, `auth_password`, and `connection` are not allowed with the `using` argument.

The return value will be the number of successfully delivered messages (which can be `0` or `1` since it can only send one message).

<div class="deprecated">

6.0

Passing `fail_silently` and later parameters as positional arguments is deprecated.

</div>

<div class="deprecated">

6.1

The `fail_silently`, `auth_user`, `auth_password`, and `connection` arguments are deprecated. In most cases they can be replaced by `using` with an appropriate `MAILERS` configuration. See `migrating-to-mailers-fail-silently`, `migrating-to-mailers-auth`, and `migrating-to-mailers-get-connection`.

</div>

<div class="versionchanged">

6.1

The `using` argument was added.

Older versions ignored `fail_silently=True`, `auth_user`, and `auth_password` when a `connection` was also provided. This now raises a `TypeError`.

</div>

### `send_mass_mail()`

<div class="function">

send_mass_mail(datatuple, \*, fail_silently=False, auth_user=None, auth_password=None, connection=None, using=None)

</div>

`django.core.mail.send_mass_mail()` is intended to handle mass emailing.

`datatuple` is a tuple in which each element is in this format:

    (subject, message, from_email, recipient_list)

`fail_silently`, `auth_user`, `auth_password` and `connection` have the same functions as in `send_mail`. They must be given as keyword arguments if used, and are not allowed with the `using` argument.

The keyword argument `using` is an optional `MAILERS` alias to use to send the mail. If unspecified, the default mailer configuration will be used.

Each separate element of `datatuple` results in a separate email message. As in `send_mail`, recipients in the same `recipient_list` will all see the other addresses in the email messages' "To:" field.

For example, the following code would send two different messages to two different sets of recipients; however, only one connection to the mail server would be opened:

    message1 = (
        "Subject here",
        "Here is the message",
        "from@example.com",
        ["first@example.com", "other@example.com"],
    )
    message2 = (
        "Another Subject",
        "Here is another message",
        "from@example.com",
        ["second@test.com"],
    )
    send_mass_mail((message1, message2))

The return value will be the number of successfully delivered messages.

<div class="deprecated">

6.0

Passing `fail_silently` and later parameters as positional arguments is deprecated.

</div>

<div class="deprecated">

6.1

The `fail_silently`, `auth_user`, `auth_password`, and `connection` arguments are deprecated. In most cases they can be replaced by `using` with an appropriate `MAILERS` configuration. See `migrating-to-mailers-fail-silently`, `migrating-to-mailers-auth`, and `migrating-to-mailers-get-connection`.

</div>

<div class="versionchanged">

6.1

The `using` argument was added.

Older versions ignored `fail_silently=True`, `auth_user`, and `auth_password` when a `connection` was also provided. This now raises a `TypeError`.

</div>

#### `send_mass_mail()` vs. `send_mail()`

The main difference between `send_mass_mail` and repeatedly calling `send_mail` is that `send_mail` opens a connection to the mail server each time it's executed, while `send_mass_mail` uses a single connection for all of its messages. This makes `send_mass_mail` slightly more efficient.

`send_mail` with multiple `to` addresses sends a single email message, with `john@example.com` and `jane@example.com` both appearing in the "To:" field:

    send_mail(
        "Subject",
        "Message.",
        "from@example.com",
        ["john@example.com", "jane@example.com"],
    )

`send_mass_mail` sends a separate message per `datatuple` element, so `john@example.com` and `jane@example.com` each receive their own email:

    datatuple = (
        ("Subject", "Message.", "from@example.com", ["john@example.com"]),
        ("Subject", "Message.", "from@example.com", ["jane@example.com"]),
    )
    send_mass_mail(datatuple)

### `mail_admins()`

<div class="function">

mail_admins(subject, message, \*, fail_silently=False, connection=None, html_message=None, using=None)

</div>

`django.core.mail.mail_admins()` is a shortcut for sending an email to the site admins, as defined in the `ADMINS` setting.

`mail_admins()` prefixes the subject with the value of the `EMAIL_SUBJECT_PREFIX` setting, which is `"[Django] "` by default.

The "From:" header of the email will be the value of the `SERVER_EMAIL` setting.

This method exists for convenience and readability.

If `html_message` is provided, the resulting email will be a `multipart/alternative` email with `message` as the `text/plain` content type and `html_message` as the `text/html` content type.

The keyword argument `using` is an optional `MAILERS` alias to use to send the mail. If unspecified, the default mailer configuration will be used.

<div class="deprecated">

6.0

Passing `fail_silently` and later parameters as positional arguments is deprecated.

</div>

<div class="deprecated">

6.1

The `fail_silently` and `connection` arguments are deprecated. In most cases they can be replaced by `using` with an appropriate `MAILERS` configuration. See `migrating-to-mailers-fail-silently` and `migrating-to-mailers-get-connection`.

</div>

<div class="versionchanged">

6.1

The `using` argument was added.

Older versions ignored `fail_silently=True` when a `connection` was also provided. This now raises a `TypeError`.

</div>

### `mail_managers()`

<div class="function">

mail_managers(subject, message, \*, fail_silently=False, connection=None, html_message=None, using=None)

</div>

`django.core.mail.mail_managers()` is just like `mail_admins()`, except it sends an email to the site managers, as defined in the `MANAGERS` setting.

The keyword argument `using` is an optional `MAILERS` alias to use to send the mail. If unspecified, the default mailer configuration will be used.

<div class="deprecated">

6.0

Passing `fail_silently` and later parameters as positional arguments is deprecated.

</div>

<div class="deprecated">

6.1

The `fail_silently` and `connection` arguments are deprecated. In most cases they can be replaced by `using` with an appropriate `MAILERS` configuration. See `migrating-to-mailers-fail-silently` and `migrating-to-mailers-get-connection`.

</div>

<div class="versionchanged">

6.1

The `using` argument was added.

Older versions ignored `fail_silently=True` when a `connection` was also provided. This now raises a `TypeError`.

</div>

### The `EmailMessage` class

Django's `send_mail` and `send_mass_mail` functions are actually thin wrappers that make use of the `EmailMessage` class.

Not all features of the `EmailMessage` class are available through the `send_mail` and related wrapper functions. If you wish to use advanced features, such as BCC'ed recipients, file attachments, or multi-part email, you'll need to create `EmailMessage` instances directly.

<div class="note">

<div class="title">

Note

</div>

This is a design feature. `send_mail` and related functions were originally the only interface Django provided. However, the list of parameters they accepted was slowly growing over time. It made sense to move to a more object-oriented design for email messages and retain the original functions only for backwards compatibility.

</div>

`EmailMessage` is responsible for creating the email message itself. The `email backend <topic-email-backends>` is then responsible for sending the email.

For convenience, `EmailMessage` provides a `~EmailMessage.send` method for sending a single email. If you need to send multiple messages, the email backend API `provides an alternative
<topics-sending-multiple-emails>`.

<div class="EmailMessage">

The `!EmailMessage` class is initialized with the following parameters. All parameters are optional and can be set at any time prior to calling the `send` method.

The first four parameters can be passed as positional or keyword arguments, but must be in the given order if positional arguments are used:

- `subject`: The subject line of the email.
- `body`: The body text. This should be a plain text message.
- `from_email`: The sender's address. Both `fred@example.com` and `"Fred" <fred@example.com>` forms are supported (see `formatting-addresses`). If omitted, the `DEFAULT_FROM_EMAIL` setting is used.
- `to`: A list or tuple of recipient addresses.

The following parameters must be given as keyword arguments if used:

- `cc`: A list or tuple of recipient addresses used in the "Cc" header when sending the email.

- `bcc`: A list or tuple of addresses used for blind carbon copies when sending the email.

- `reply_to`: A list or tuple of recipient addresses used in the "Reply-To" header when sending the email.

- `attachments`: A list of attachments to put on the message. Each can be an instance of `~email.message.MIMEPart` or `EmailAttachment`, or a tuple with attributes `(filename, content, mimetype)`.

  <div class="versionchanged">

  6.0

  Support for `~email.message.MIMEPart` objects in the `attachments` list was added.

  </div>

  <div class="deprecated">

  6.0

  Support for Python's legacy `~email.mime.base.MIMEBase` objects in `attachments` is deprecated. Use `~email.message.MIMEPart` instead.

  </div>

- `headers`: A dictionary of extra headers to put on the message. The keys are the header name, values are the header values. It's up to the caller to ensure header names and values are in the correct format for an email message. The corresponding attribute is `extra_headers`.

- `connection`: An `email backend <topic-email-backends>` instance. This parameter is ignored when using `send_messages() <topics-sending-multiple-emails>`.

  <div class="deprecated">

  6.1

  The `connection` argument is deprecated. Instead, define a `MAILERS` configuration with the desired connection options, and then call `EmailMessage.send(using="...") <send>` with that configuration's alias. See `migrating-to-mailers`.

  </div>

<div class="deprecated">

6.0

Passing all except the first four parameters as positional arguments is deprecated.

</div>

For example:

    from django.core.mail import EmailMessage

    email = EmailMessage(
        subject="Hello",
        body="Body goes here",
        from_email="from@example.com",
        to=["to1@example.com", "to2@example.com"],
        bcc=["bcc@example.com"],
        reply_to=["another@example.com"],
        headers={"Message-ID": "foo"},
    )

The class has the following methods:

<div class="method">

send(fail_silently=False, \*, using=None)

Sends the message. Returns `1` if the message was sent successfully, otherwise `0`. (An empty list of recipients returns `0` -- it will not raise an exception.)

The optional `using` keyword argument specifies a `MAILERS` alias to use to send the mail. If not given, the default mailer configuration will be used.

If a deprecated connection was specified when the email was constructed, that connection will be used. Providing both a connection and `using` will raise an error.

If the deprecated keyword argument `fail_silently` is `True`, certain backend-dependent exceptions while sending the message will be ignored. Providing both `fail_silently` and `using` will raise an error.

<div class="versionchanged">

6.1

The `using` argument was added.

Older versions ignored `fail_silently=True` when a `connection` was also provided. This now raises a `TypeError`.

</div>

<div class="deprecated">

6.1

The `fail_silently` argument is deprecated. See `migrating-to-mailers-fail-silently` for alternatives.

</div>

</div>

<div class="method">

message(\*, policy=email.policy.default)

Constructs and returns a Python `email.message.EmailMessage` object representing the message to be sent.

The keyword argument `policy` allows specifying the set of rules for updating and serializing the representation of the message. It must be an `email.policy.Policy <email.policy>` object. Defaults to `email.policy.default`. In certain cases you may want to use `~email.policy.SMTP`, `~email.policy.SMTPUTF8` or a custom policy. For example, the `SMTP email backend
<topic-email-smtp-backend>` uses the `~email.policy.SMTP` policy to ensure `\r\n` line endings as required by the SMTP protocol.

If you ever need to extend Django's `EmailMessage` class, you'll probably want to override this method to put the content you want into the Python EmailMessage object.

<div class="versionchanged">

6.0

The `policy` keyword argument was added and the return type was updated to an instance of `~email.message.EmailMessage`.

</div>

</div>

<div class="method">

recipients()

Returns a list of all the recipients of the message, whether they're recorded in the `to`, `cc` or `bcc` attributes. This is another method you might need to override when subclassing, because the SMTP server needs to be told the full list of recipients when the message is sent. If you add another way to specify recipients in your class, they need to be returned from this method as well.

</div>

<div class="method">

attach(filename, content, mimetype) attach(mimepart)

Creates a new attachment and adds it to the message. There are two ways to call `!attach`:

- You can pass it three arguments: `filename`, `content` and `mimetype`. `filename` is the name of the file attachment as it will appear in the email, `content` is the data that will be contained inside the attachment and `mimetype` is the optional MIME type for the attachment. If you omit `mimetype`, the MIME content type will be guessed from the filename of the attachment.

  For example:

      message.attach("design.png", img_data, "image/png")

  If you specify a `mimetype` of `message/rfc822`, `content` can be a `django.core.mail.EmailMessage` or Python's `email.message.EmailMessage` or `email.message.Message`.

  For a `mimetype` starting with `text/`, content is expected to be a string. Binary data will be decoded using UTF-8, and if that fails, the MIME type will be changed to `application/octet-stream` and the data will be attached unchanged.

- Or for attachments requiring additional headers or parameters, you can pass `!attach` a single Python `~email.message.MIMEPart` object. This will be attached directly to the resulting message. For example, to attach an inline image with a `Content-ID`:

      import email.utils
      from email.message import MIMEPart
      from django.core.mail import EmailMultiAlternatives

      message = EmailMultiAlternatives(...)
      image_data_bytes = ...  # Load image as bytes

      # Create a random Content-ID, including angle brackets
      cid = email.utils.make_msgid()
      inline_image = email.message.MIMEPart()
      inline_image.set_content(
          image_data_bytes,
          maintype="image",
          subtype="png",  # or "jpeg", etc. depending on the image type
          disposition="inline",
          cid=cid,
      )
      message.attach(inline_image)
      # Refer to Content-ID in HTML without angle brackets
      message.attach_alternative(f'… <img src="cid:{cid[1:-1]}"> …', "text/html")

  Python's `email.contentmanager.set_content` documentation describes the supported arguments for `MIMEPart.set_content()`.

  <div class="versionchanged">

  6.0

  Support for `~email.message.MIMEPart` attachments was added.

  </div>

  <div class="deprecated">

  6.0

  Support for `email.mime.base.MIMEBase` attachments is deprecated. Use `~email.message.MIMEPart` instead.

  </div>

</div>

<div class="method">

attach_file(path, mimetype=None)

Creates a new attachment using a file from your filesystem. Call it with the path of the file to attach and, optionally, the MIME type to use for the attachment. If the MIME type is omitted, it will be guessed from the filename. You can use it like this:

    message.attach_file("/images/weather_map.png")

For MIME types starting with `text/`, binary data is handled as in `attach`.

</div>

</div>

<div class="EmailAttachment">

A named tuple to store attachments to an email.

The named tuple has the following indexes:

- `filename`
- `content`
- `mimetype`

</div>

#### Sending alternative content types

##### Sending multiple content versions

It can be useful to include multiple versions of the content in an email; the classic example is to send both text and HTML versions of a message. With Django's email library, you can do this using the `EmailMultiAlternatives` class.

<div class="EmailMultiAlternatives">

A subclass of `EmailMessage` that allows additional versions of the message body in the email via the `attach_alternative` method. This directly inherits all methods (including the class initialization) from `EmailMessage`.

<div class="attribute">

alternatives

A list of `EmailAlternative` named tuples. This is particularly useful in tests:

    self.assertEqual(len(msg.alternatives), 1)
    self.assertEqual(msg.alternatives[0].content, html_content)
    self.assertEqual(msg.alternatives[0].mimetype, "text/html")

Alternatives should only be added using the `attach_alternative` method, or passed to the constructor.

</div>

<div class="method">

attach_alternative(content, mimetype)

Attach an alternative representation of the message body in the email.

For example, to send a text and HTML combination, you could write:

    from django.core.mail import EmailMultiAlternatives

    subject = "hello"
    from_email = "from@example.com"
    to = "to@example.com"
    text_content = "This is an important message."
    html_content = "<p>This is an <strong>important</strong> message.</p>"
    msg = EmailMultiAlternatives(subject, text_content, from_email, [to])
    msg.attach_alternative(html_content, "text/html")
    msg.send()

</div>

<div class="method">

body_contains(text)

Returns a boolean indicating whether the provided `text` is contained in the email `body` and in all attached MIME type `text/*` alternatives.

This can be useful when testing emails. For example:

    def test_contains_email_content(self):
        subject = "Hello World"
        from_email = "from@example.com"
        to = "to@example.com"
        msg = EmailMultiAlternatives(subject, "I am content.", from_email, [to])
        msg.attach_alternative("<p>I am content.</p>", "text/html")

        self.assertIs(msg.body_contains("I am content"), True)
        self.assertIs(msg.body_contains("<p>I am content.</p>"), False)

</div>

</div>

<div class="EmailAlternative">

A named tuple to store alternative versions of email content.

The named tuple has the following indexes:

- `content`
- `mimetype`

</div>

##### Updating the default content type

By default, the MIME type of the `body` parameter in an `EmailMessage` is `"text/plain"`. It is good practice to leave this alone, because it guarantees that any recipient will be able to read the email, regardless of their mail client. However, if you are confident that your recipients can handle an alternative content type, you can use the `content_subtype` attribute on the `EmailMessage` class to change the main content type. The major type will always be `"text"`, but you can change the subtype. For example:

    msg = EmailMessage(subject, html_content, from_email, [to])
    msg.content_subtype = "html"  # Main content is now text/html
    msg.send()

### Safely sending email

Any public website that can send email will eventually be targeted by attempts to abuse it for spam, phishing, or other malicious content. While a complete discussion of vulnerabilities in sending email is beyond the scope of Django's documentation, there are many references available on the web. Two good starting points are:

- Princeton University's guidance on [preventing email abuse in web forms](https://sitebuilder.princeton.edu/site-administration/spam). Although this is an internal reference for users of Princeton's Drupal Site Builder, nearly all of its advice applies equally to sites built with Django (or any web framework).
- OWASP's [Email Validation and Verification in Identity Systems Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Email_Validation_and_Verification_Cheat_Sheet.html). This primarily covers using email in authentication contexts. While many of its recommendations are handled by `django.contrib.auth` and other Django features, items like rate limiting and securing email change workflows are the developer's responsibility.

Thinking through how email might be abused (and taking steps to mitigate it) is especially important if your site can send to unverified addresses. Features like newsletter sign-up, contact forms that cc or auto-reply to the sender, and "share this page" can be attractive targets.

#### Formatting email addresses

Email addresses allow a "friendly" display name alongside the `user@domain` address. For example, you could include your company name in the `DEFAULT_FROM_EMAIL` setting:

    DEFAULT_FROM_EMAIL = '"Example, Inc." <contact@example.com>'

The double quotes around `"Example, Inc."` are needed so the comma isn't read as separating two different addresses. A fixed address, written out by hand as in the example above, is safe, but composing one from variable parts (especially untrusted input) needs more care.

<div class="warning">

<div class="title">

Warning

</div>

Never use string formatting to build an email address from variable parts. For example, `f'"{name}" <{email}>'` is **unsafe**.

</div>

Email address headers have complex syntax rules (much like HTML or SQL), so constructing them by combining strings creates an injection vulnerability. Even if you've `validated <.EmailValidator>` the format of `email`, an attacker could exploit the `name` portion to inject additional addresses.

To avoid this, always use a well-tested library specifically meant to format email addresses, like Python's `email.headerregistry.Address` class (the replacement for the legacy `~email.utils.formataddr` function, which does not support internationalized domain names).

For example, to include a user's full name when sending them email (where `user` is an instance of the default `.User` model):

    from django.core.mail import send_mail
    from email.headerregistry import Address


    def send_mail_to_user(user, subject, body, from_email=None):
        # Safely create an email address with the user's name.
        # (addr_spec is the technical term for the user@domain address.)
        address = Address(
            display_name=user.get_full_name(),
            addr_spec=user.email,
        )
        send_mail(subject, body, from_email, [address])

Django's built-in email backends support using `~email.headerregistry.Address` objects directly in any address field, as shown here. So do many custom and third-party email backends. But if this causes a `TypeError` or other problem with a particular backend, use `str(address)` to convert the object to a safe, properly formatted string.

#### Preventing header injection

[Email header injection](https://en.wikipedia.org/wiki/Email_injection) is a security exploit in which an attacker manipulates email headers to change the intended sender or recipients, subject, or potentially even the entire visible message body.

One type of header injection exploits address header syntax. You are responsible for preventing this when constructing email addresses from user-supplied input, as described in `formatting-addresses` above.

Another (perhaps better understood) attack, CRLF injection, uses carriage return and line feed characters to insert additional headers into the email. Django prevents this by raising a `ValueError` if those characters appear in any header field when trying to send the message.

<div class="versionchanged">

6.0

Older versions raised `django.core.mail.BadHeaderError` for some invalid headers. This has been replaced with `!ValueError`.

</div>

Django's CRLF protection relies on using Python's modern `~email.policy.EmailPolicy` in Django's `EmailMessage.message`. Custom email backends that don't call that function, or that call it with the legacy `~email.policy.compat32` policy, are responsible for implementing their own CRLF injection prevention.

### Sending many messages efficiently

Establishing and closing an SMTP connection (or any other network connection, for that matter) is an expensive process. If you have a lot of emails to send, it makes sense to reuse an SMTP connection, rather than creating and destroying a connection every time you want to send an email.

There are two ways to tell an email backend to reuse a connection. Both involve obtaining an email backend instance from `.mail.mailers` and using the `backend's API <topic-email-backend-api>`.

The first approach is to use the backend's `send_messages()` method. This takes a list of `EmailMessage` (or subclass) instances, and sends them all using that single connection.

For example, if you have a function called `get_notification_emails()` that returns a list of `EmailMessage` objects representing some periodic email you wish to send out, you could send these emails using a single call to `send_messages()`:

    from django.core import mail

    email_list = get_notification_emails()
    # Use the default mailer. You could substitute
    # mail.mailers["alias"] for a specific mailer.
    backend = mail.mailers.default
    backend.send_messages(email_list)

In this example, the call to `send_messages()` opens a connection on the backend, sends the list of messages, and then closes the connection again. (This is how `send_mass_mail` is implemented.)

The second approach is to use the `open()` and `close()` methods on the email backend to manually control the connection. `send_messages()` will not open or close the connection if it is already open, so if you manually open the connection, you can control when it is closed. For example:

    from django.core import mail

    # Use the "notifications" mailer configuration.
    backend = mail.mailers["notifications"]

    # Manually open the connection.
    backend.open()

    # Construct an email message. (Passing None as the third argument
    # uses settings.DEFAULT_FROM_EMAIL as the "From:" address.)
    email1 = mail.EmailMessage("Hi", "Message", None, ["to1@example.com"])
    # Send the email. The connection was already open, so send_messages()
    # leaves it open after sending.
    backend.send_messages([email1])

    # Construct and send two more messages. The connection is still open.
    email2 = mail.EmailMessage("Hi", "Message", None, ["to2@example.com"])
    email3 = mail.EmailMessage("Hi", "Message", None, ["to3@example.com"])
    backend.send_messages([email2, email3])

    # Because we opened it, we need to manually close the connection.
    backend.close()

When you manually open a backend's connection, you are responsible for ensuring it gets closed. The example above actually has a bug: if an exception occurs while sending the messages, the connection will not be closed. This can be fixed with a `try`-`finally` statement, but using the backend instance as a context manager is preferable, as it automatically calls `open()` and `close()` as needed.

This is equivalent to the previous example, but uses the backend as a `context manager <context-managers>` to avoid leaving the connection open on errors:

    from django.core import mail

    # Use mail.mailers[...] as a context manager.
    with mail.mailers["notifications"] as backend:
        # The backend connection is automatically opened inside the context.
        email1 = mail.EmailMessage("Hi", "Message", None, ["to1@example.com"])
        backend.send_messages([email1])

        # The connection is still open, and is reused for the second send.
        email2 = mail.EmailMessage("Hi", "Message", None, ["to2@example.com"])
        email3 = mail.EmailMessage("Hi", "Message", None, ["to3@example.com"])
        backend.send_messages([email2, email3])

    # After exiting the context (either normally or because of an error),
    # the backend connection is automatically closed.

## Email backends

The actual sending of an email is handled by the email backend.

Django comes with several email backends. With the exception of the SMTP backend, these are mainly useful during testing and development. If the built-in backends don't meet your needs there are `third-party packages
<topic-third-party-email-backends>` available. You can also subclass one of the built-in backends to change its behavior, or even `write your own email
backend <topic-custom-email-backend>`.

### SMTP backend

The SMTP email backend connects to an SMTP server to send email. To use it, set `BACKEND <MAILERS-BACKEND>` to `"django.core.mail.backends.smtp.EmailBackend"`.

The SMTP backend supports these `OPTIONS <MAILERS-OPTIONS>`:

- `"host"` (required): the SMTP server hostname or IP address.

- `"port"`: the port number to connect to on the SMTP host. If omitted, uses the standard port for the connection protocol depending on the `"use_tls"` and `"use_ssl"` options: `587` for TLS, `465` for SSL, or `25` for an unsecured connection.

- `"username"` and `"password"`: set these if your server requires SMTP authentication ("SMTP AUTH" credentials, sometimes called SMTP login).

  Although the username is often an email address, it should not be confused with default "From:" addresses. Those are defined by the `DEFAULT_FROM_EMAIL` and `SERVER_EMAIL` settings.

- `"use_tls"` or `"use_ssl"`: set one of these options to `True` to connect to the SMTP server using a secure protocol -- `"use_tls"` for explicit TLS or `"use_ssl"` for SSL (implicit TLS).

- `"ssl_certfile"` and `"ssl_keyfile"`: if the SMTP server's SSL/TLS connection requires client certificate authentication, use these options to specify the paths to a PEM-formatted certificate chain file and private key file. (The key file can be omitted if the certificate file includes the private key.)

  These options are not intended for use with a private certificate authority or self-signed SMTP server certificate. See `topic-email-smtp-private-ca` below.

  Note that these options don't result in checking certificate validity. They are passed to the underlying SSL connection. Refer to the documentation of Python's `ssl.SSLContext.wrap_socket` method for details on how the certificate chain file and private key file are handled.

- `"timeout"`: the timeout (in seconds) for connecting to the SMTP server and other blocking operations. If not specified, the value is obtained from `socket.getdefaulttimeout`, which defaults to no timeout (`None`) meaning SMTP operations can block indefinitely.

- `"fail_silently"`: set to `True` to ignore certain errors while sending a message. All `OSError`s are ignored while opening the SMTP connection, and `smtplib.SMTPException` errors are ignored while communicating with the server. This will suppress both transient network glitches and also serious configuration problems. However, it does not ignore *all* errors, and problems with serializing the message will not fail silently. (This option is available for backward compatibility but is not recommended for typical use.)

Example:

    MAILERS = {
        "default": {
            "BACKEND": "django.core.mail.backends.smtp.EmailBackend",
            "OPTIONS": {
                "host": "smtp.example.net",
                "use_tls": True,
                "username": "my-app",
                "password": os.environ["MY_APP_SMTP_PASSWORD"],
                "timeout": 10,
            },
        },
    }

<div class="deprecated">

6.1

When the `MAILERS` setting is not defined, Django uses the SMTP backend as the default mailer (the default `EMAIL_BACKEND`), connecting to localhost on port 25. This behavior will be removed in Django 7.0, which will not have a default mailer configuration.

When the SMTP backend is used without `MAILERS` defined, the options listed above are obtained from the deprecated `EMAIL_HOST`, `EMAIL_PORT`, `EMAIL_HOST_USER`, `EMAIL_HOST_PASSWORD`, `EMAIL_USE_TLS`, `EMAIL_USE_SSL`, `EMAIL_SSL_KEYFILE`, `EMAIL_SSL_CERTFILE`, and `EMAIL_TIMEOUT` settings, respectively. (There is no setting equivalent to the `"fail_silently"` option.)

</div>

<div class="backends.smtp.EmailBackend">

Directly instantiating an `EmailBackend` class is not recommended. Use `mailers` to obtain a backend instance.

When constructed directly (without going through `mailers`), the SMTP `EmailBackend` class accepts the options listed above as keyword arguments. Default values come from the corresponding, deprecated `EMAIL_*` settings. `host` is not required and defaults to `"localhost"`, and `port` defaults to `25` even if `use_tls` or `use_ssl` is True.

When the `MAILERS` setting is defined, attempting to directly create an SMTP `EmailBackend` will raise an `AttributeError`.

<div class="deprecated">

6.1

Directly constructing an instance of an `EmailBackend` class will be unsupported in Django 7.0. Undocumented use will result in different default argument handling compared to earlier releases.

</div>

</div>

#### Private and self-signed SMTP server certificates

If the SMTP server uses an SSL certificate from a private certificate authority (CA), the CA's root certificate should be added to the system CA bundle on the client (where Django is running). Likewise, if the server uses a self-signed certificate, it should be added to the client's system CA bundle so it can be trusted. (The SMTP backend's `"ssl_certfile"` option cannot be used for CA roots or self-signed certificates.)

Follow platform-specific instructions for adding to the system CA bundle. If modifying the system bundle is not possible or desired, an alternative is using OpenSSL's `SSL_CERT_FILE` or `SSL_CERT_DIR` environment variables to specify a custom certificate bundle.

For more complex scenarios, the SMTP backend can be subclassed to add root certificates to its `ssl_context` using `ssl.SSLContext.load_verify_locations`.

### Console backend

Instead of sending out real emails, the console backend writes the emails that would be sent to the standard output. To use it, set `BACKEND
<MAILERS-BACKEND>` to `"django.core.mail.backends.console.EmailBackend"`.

The console backend supports these `OPTIONS <MAILERS-OPTIONS>`:

- `"stream"`: a stream-like object to write to. Defaults to `stdout`.
- `"fail_silently"`: set to `True` to ignore *all* errors while writing the message to the stream, including errors serializing the message. (This option is available for backward compatibility but is not recommended.)

This backend is not intended for use in production -- it is provided as a convenience that can be used during development.

<div class="versionchanged">

6.1

The settings file created by `startproject` now defines `MAILERS` with the console backend as the default configuration.

</div>

### File backend

The file backend writes emails to a file. A new file is created for each new session that is opened on this backend. To use it, set `BACKEND
<MAILERS-BACKEND>` to `"django.core.mail.backends.filebased.EmailBackend"`.

The file backend supports these `OPTIONS <MAILERS-OPTIONS>`:

- `"file_path"` (required): the directory to which the files are written. Can be a string or a `pathlib.Path` object. If the directory does not exist, the file backend will attempt to create it.
- `"fail_silently"`: set to `True` to ignore *all* errors while writing the message to the file -- including errors serializing the message -- but *not* errors related to ensuring the file path directory exists. (This option is available for backward compatibility but is not recommended.)

This backend is not intended for use in production -- it is provided as a convenience that can be used during development.

<div class="deprecated">

6.1

When the file backend is used without the `MAILERS` setting defined, it will get its `file_path` option from the `EMAIL_FILE_PATH` setting.

</div>

### In-memory backend

The `'locmem'` backend stores messages in a special attribute of the `django.core.mail` module. The `outbox` attribute is created when the first message is sent. It's a list with an `EmailMessage` instance for each message that would be sent. Messages in the outbox are annotated with a `sent_using` attribute that identifies the `MAILERS` alias used to send the message.

To use the in-memory backend, set `BACKEND <MAILERS-BACKEND>` to `"django.core.mail.backends.locmem.EmailBackend"`. It does not support any `OPTIONS <MAILERS-OPTIONS>`.

Django's test runner `automatically switches to this backend for testing
<topics-testing-email>`.

This backend is not intended for use in production -- it is provided as a convenience that can be used during development and testing.

<div class="versionchanged">

6.1

The `sent_using` attribute was added to messages in the outbox.

</div>

### Dummy backend

As the name suggests the dummy backend does nothing with your messages. To use it, set `BACKEND <MAILERS-BACKEND>` to `"django.core.mail.backends.dummy.EmailBackend"`. It does not support any `OPTIONS <MAILERS-OPTIONS>`.

This backend is not intended for use in production -- it is provided as a convenience that can be used during development.

### Third-party backends

<div class="admonition">

There are community-maintained solutions!

Django has a vibrant ecosystem. There are email backends highlighted on the [Community Ecosystem](https://www.djangoproject.com/community/ecosystem/#email-notifications) page. The Django Packages [Email grid](https://djangopackages.org/grids/g/email/) has even more options for you!

</div>

Third-party email backends are available that:

- Integrate directly with commercial email service providers' APIs (which often have extra functionality not available through SMTP).
- Offload email sending to asynchronous task queues.
- Add features to other email backends, such as enforcing do-not-send lists or logging sent messages.
- Provide development and debugging tools, such as sandbox capture and in-browser email previews.

### Defining a custom email backend

If you need to change how emails are sent you can write your own email backend. To use a custom backend, set `BACKEND <MAILERS-BACKEND>` to the Python import path for your backend class and `OPTIONS <MAILERS-OPTIONS>` to any `__init__()` keyword arguments your backend supports.

Custom email backends should subclass `BaseEmailBackend` that is located in the `django.core.mail.backends.base` module. A custom email backend must implement the `send_messages(email_messages)` method. This method receives a list of `EmailMessage` instances and returns the number of successfully delivered messages. If your backend has any concept of a persistent session or connection, you should also implement the `open()` and `close()` methods. Refer to `smtp.EmailBackend` for a reference implementation.

### Obtaining an instance of an email backend

The `mailers` factory in `!django.core.mail` returns instances of email backends.

<div class="data">

mailers

You can access the mailers configured in the `MAILERS` setting through a dict-like object: `django.core.mail.mailers`:

``` pycon
>>> from django.core.mail import mailers
>>> mailers["notifications"]
```

If the named key is not defined, a `MailerDoesNotExist` error will be raised. Other configuration problems will raise an `InvalidMailer` error.

<div class="versionadded">

6.1

</div>

</div>

<div class="data">

mailers.default

As a shortcut, the default mailer can be accessed through `django.core.mail.mailers.default`:

``` pycon
>>> from django.core.mail import mailers
>>> mailers.default
```

This is equivalent to `mailers["default"]`. If no default mailer has been configured, a `MailerDoesNotExist` error will be raised.

<div class="deprecated">

6.1

If the `MAILERS` setting is not defined, `mailers.default` will create an email backend instance from the deprecated `EMAIL_BACKEND` and related settings. This supports backward compatibility with Django 6.0 and earlier.

This behavior (and those settings) will be removed in Django 7.0.

</div>

</div>

<div class="function">

get_connection(backend=None, *, fail_silently=False,*\*kwargs)

The deprecated `!django.core.mail.get_connection` function creates and returns an instance of an email backend. Its behavior depends on the `MAILERS` setting and how the function is called.

If the `MAILERS` setting *is* defined:

- `get_connection()` with no arguments will return `mailers.default`.
- `get_connection(...)` called with only `fail_silently` or other keyword arguments will create an instance of `MAILERS["default"]` with any keywords added to the default mailer's `OPTIONS <MAILERS-OPTIONS>`.
- `get_connection(backend, ...)` with a backend import path will raise an error.

If the `MAILERS` setting is *not* defined:

- `get_connection()` with no arguments will return an instance of the email backend specified in `EMAIL_BACKEND`.
- If you specify the `backend` argument, an instance of that backend will be instantiated.
- If the keyword argument `fail_silently` is True, certain backend-dependent exceptions during the email sending process will be silently ignored.
- All other keyword arguments are passed directly to the constructor of the email backend.

<div class="deprecated">

6.0

Passing `fail_silently` as positional argument is deprecated.

</div>

<div class="deprecated">

6.1

`!get_connection` is deprecated and will be removed in Django 7.0. Switch to `mailers[alias] <mailers>`. See `migrating-to-mailers-get-connection` for migration suggestions.

</div>

</div>

### Email backend API

Instances of an email backend class have the following methods:

- `open()` instantiates a long-lived email-sending connection.
- `close()` closes the current email-sending connection.
- `send_messages(email_messages)` sends a list of `EmailMessage` objects. If the connection is not open, this call will implicitly open the connection, and close the connection afterward. If the connection is already open, it will be left open after mail has been sent.

A backend instance can also be used as a context manager, which will automatically call `open()` and `close()` as needed. An example is in `topics-sending-multiple-emails`.

## Configuring email for development

There are times when you do not want Django to send emails at all. For example, while developing a website, you probably don't want to send out thousands of emails -- but you may want to validate that emails will be sent to the right people under the right conditions, and that those emails will contain the correct content.

The easiest way to configure email for local development is to use the `console <topic-email-console-backend>` email backend. This backend redirects all email to `stdout`, allowing you to inspect the content of mail.

The `file <topic-email-file-backend>` email backend can also be useful during development -- this backend dumps the contents of every SMTP connection to a file that can be inspected at your leisure.

Another approach is to use a mocked SMTP server that receives the emails locally and displays them to the terminal, but does not actually send anything. The `aiosmtpd` package provides a way to accomplish this:

``` shell
python -m pip install "aiosmtpd >= 1.4.5"

python -m aiosmtpd -n -l localhost:8025
```

This command will start a minimal SMTP server listening on port 8025 of localhost. This server prints to standard output all email headers and the email body. You then only need to set an SMTP backend's `"host"` and `"port"` `OPTIONS <MAILERS-OPTIONS>` accordingly. For a more detailed discussion of SMTP server options, see the documentation of the [aiosmtpd](https://aiosmtpd.readthedocs.io/en/latest/) module.

For information about unit-testing the sending of emails in your application, see the `topics-testing-email` section of the testing documentation.
