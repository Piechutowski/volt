# Unit tests

Django comes with a test suite of its own, in the `tests` directory of the code base. It's our policy to make sure all tests pass at all times.

We appreciate any and all contributions to the test suite!

The Django tests all use the testing infrastructure that ships with Django for testing applications. See `/topics/testing/overview` for an explanation of how to write new tests.

## Running the unit tests

### Quickstart

First, [fork Django on GitHub](https://github.com/django/django/fork).

Second, create and activate a virtual environment. If you're not familiar with how to do that, read our `contributing tutorial </intro/contributing>`.

Next, clone your fork, install some requirements, and run the tests:

<div class="console">

\$ git clone <https://github.com/YourGitHubName/django.git> django-repo \$ cd django-repo/tests \$ python -m pip install -e .. \$ python -m pip install -r requirements/py3.txt \$ ./runtests.py

</div>

Installing the requirements will likely require some operating system packages that your computer doesn't have installed. You can usually figure out which package to install by doing a web search for the last line or so of the error message. Try adding your operating system to the search query if needed.

If you have trouble installing an optional test dependency, you can skip that dependency locally. For example, if installing `pylibmc` fails, comment out its line in `requirements/py3.txt` and continue with `./runtests.py`. Tests that require `pylibmc` will be skipped automatically. See `running-unit-tests-dependencies` for details on installing the optional test dependencies.

Running the tests requires a Django settings module that defines the databases to use. To help you get started, Django provides and uses a sample settings module that uses the SQLite database. See `running-unit-tests-settings` to learn how to use a different settings module to run the tests with a different database.

Having problems? See `troubleshooting-unit-tests` for some common issues.

### Running tests using `tox`

[Tox](https://tox.wiki/) is a tool for running tests in different virtual environments. Django includes a basic `tox.ini` that automates some checks that our build server performs on pull requests. To run the unit tests and other checks (such as `import sorting <coding-style-imports>`, the `documentation spelling checker <documentation-spelling-check>`, and `code formatting <coding-style-python>`), install and run the `tox` command from any place in the Django source tree:

<div class="console">

\$ python -m pip install tox \$ tox

</div>

By default, `tox` runs the test suite with the bundled test settings file for SQLite, `black`, `blacken-docs`, `flake8`, `isort`, `lint-docs`, `zizmor`, and the documentation spelling checker. In addition to the system dependencies noted elsewhere in this documentation, the command `python3` must be on your path and linked to the appropriate version of Python. A list of default environments can be seen as follows:

<div class="console">

\$ tox -l py3 black blacken-docs flake8\>=3.7.0 docs isort\>=7.0.0 lint-docs zizmor\>=1.19.0

</div>

#### Testing other Python versions and database backends

In addition to the default environments, `tox` supports running unit tests for other versions of Python and other database backends. Since Django's test suite doesn't bundle a settings file for database backends other than SQLite, however, you must `create and provide your own test settings
<running-unit-tests-settings>`. For example, to run the tests on Python 3.12 using PostgreSQL:

<div class="console">

\$ tox -e py312-postgres -- --settings=my_postgres_settings

</div>

This command sets up a Python 3.12 virtual environment, installs Django's test suite dependencies (including those for PostgreSQL), and calls `runtests.py` with the supplied arguments (in this case, `--settings=my_postgres_settings`).

The remainder of this documentation shows commands for running tests without `tox`, however, any option passed to `runtests.py` can also be passed to `tox` by prefixing the argument list with `--`, as above.

`Tox` also respects the `DJANGO_SETTINGS_MODULE` environment variable, if set. For example, the following is equivalent to the command above:

``` console
$ DJANGO_SETTINGS_MODULE=my_postgres_settings tox -e py312-postgres
```

Windows users should use:

``` doscon
...\> set DJANGO_SETTINGS_MODULE=my_postgres_settings
...\> tox -e py312-postgres
```

#### Running the JavaScript tests

Django includes a set of `JavaScript unit tests <javascript-tests>` for functions in certain contrib apps. The JavaScript tests aren't run by default using `tox` because they require `Node.js` to be installed and aren't necessary for the majority of patches. To run the JavaScript tests using `tox`:

<div class="console">

\$ tox -e javascript

</div>

This command runs `npm install` to ensure test requirements are up to date and then runs `npm test`.

### Running tests using `django-docker-box`

[django-docker-box](https://github.com/django/django-docker-box/) allows you to run the Django's test suite across all supported databases and python versions. See the [django-docker-box](https://github.com/django/django-docker-box/) project page for installation and usage instructions.

### Using another `settings` module

The included settings module (`tests/test_sqlite.py`) allows you to run the test suite using SQLite. If you want to run the tests using a different database, you'll need to define your own settings file. Some tests, such as those for `contrib.postgres`, are specific to a particular database backend and will be skipped if run with a different backend. Some tests are skipped or expected failures on a particular database backend (see `DatabaseFeatures.django_test_skips` and `DatabaseFeatures.django_test_expected_failures` on each backend).

To run the tests with different settings, ensure that the module is on your `PYTHONPATH` and pass the module with `--settings`.

The `DATABASES` setting in any test settings module needs to define two databases:

- A `default` database. This database should use the backend that you want to use for primary testing.
- A database with the alias `other`. The `other` database is used to test that queries can be directed to different databases. This database should use the same backend as the `default`, and it must have a different name.

If you're using a backend that isn't SQLite, you will need to provide other details for each database:

- The `USER` option needs to specify an existing user account for the database. That user needs permission to execute `CREATE DATABASE` so that the test database can be created.
- The `PASSWORD` option needs to provide the password for the `USER` that has been specified.

Test databases get their names by prepending `test_` to the value of the `NAME` settings for the databases defined in `DATABASES`. These test databases are deleted when the tests are finished.

You will also need to ensure that your database uses UTF-8 as the default character set. If your database server doesn't use UTF-8 as a default charset, you will need to include a value for `CHARSET <TEST_CHARSET>` in the test settings dictionary for the applicable database.

### Running only some of the tests

Django's entire test suite takes a while to run, and running every single test could be redundant if, say, you just added a test to Django that you want to run quickly without running everything else. You can run a subset of the unit tests by appending the names of the test modules to `runtests.py` on the command line.

For example, if you'd like to run tests only for generic relations and internationalization, type:

<div class="console">

\$ ./runtests.py --settings=path.to.settings generic_relations i18n

</div>

How do you find out the names of individual tests? Look in `tests/` — each directory name there is the name of a test.

If you want to run only a particular class of tests, you can specify a list of paths to individual test classes. For example, to run the `TranslationTests` of the `i18n` module, type:

<div class="console">

\$ ./runtests.py --settings=path.to.settings i18n.tests.TranslationTests

</div>

Going beyond that, you can specify an individual test method like this:

<div class="console">

\$ ./runtests.py --settings=path.to.settings i18n.tests.TranslationTests.test_lazy_objects

</div>

You can run tests starting at a specified top-level module with `--start-at` option. For example:

<div class="console">

\$ ./runtests.py --start-at=wsgi

</div>

You can also run tests starting after a specified top-level module with `--start-after` option. For example:

<div class="console">

\$ ./runtests.py --start-after=wsgi

</div>

Note that the `--reverse` option doesn't impact on `--start-at` or `--start-after` options. Moreover these options cannot be used with test labels.

### Running the Selenium tests

Some tests require Selenium and a web browser. To run these tests, you must install the `selenium` package and run the tests with the `--selenium=<BROWSERS>` option. For example, if you have Firefox and Google Chrome installed:

<div class="console">

\$ ./runtests.py --selenium=firefox,chrome

</div>

See the [selenium.webdriver](https://github.com/SeleniumHQ/selenium/tree/trunk/py/selenium/webdriver) package for the list of available browsers.

Specifying `--selenium` automatically sets `--tags=selenium` to run only the tests that require selenium.

Some browsers (e.g. Chrome or Firefox) support headless testing, which can be faster and more stable. Add the `--headless` option to enable this mode.

#### Screenshot tests

For testing changes to the admin UI, the selenium tests can be run with the `--screenshots` option enabled. Screenshots will be saved to the `tests/screenshots/` directory.

To define when screenshots should be taken during a selenium test, the test class must use the `@django.test.selenium.screenshot_cases` decorator with a list of supported screenshot types (`"desktop_size"`, `"mobile_size"`, `"small_screen_size"`, `"rtl"`, `"dark"`, and `"high_contrast"`). It can then call `self.take_screenshot("unique-screenshot-name")` at the desired point to generate the screenshots. For example:

    from django.test.selenium import SeleniumTestCase, screenshot_cases
    from django.urls import reverse


    class SeleniumTests(SeleniumTestCase):
        @screenshot_cases(["desktop_size", "mobile_size", "rtl", "dark", "high_contrast"])
        def test_login_button_centered(self):
            self.selenium.get(self.live_server_url + reverse("admin:login"))
            self.take_screenshot("login")
            ...

This generates multiple screenshots of the login page - one for a desktop screen, one for a mobile screen, one for right-to-left languages on desktop, one for the dark mode on desktop, and one for high contrast mode on desktop when using chrome.

### Running all the tests

If you want to run the full suite of tests, you'll need to install a number of dependencies:

- `aiosmtpd` 1.4.5+
- `argon2-cffi` 23.1.0+
- `asgiref` 3.9.1+ (required)
- `bcrypt` 4.1.1+
- `colorama` 0.4.6+
- `docutils` 0.22+
- `geoip2` 4.8.0+
- `Jinja2` 2.11+
- `numpy` 1.26.0+
- `Pillow` 10.1.0+
- `PyYAML` 6.0.2+
- `pywatchman`
- `redis` 5.1.0+
- `pymemcache`, plus a [supported Python binding](https://memcached.org/)
- [gettext](https://www.gnu.org/software/gettext/manual/gettext.html) (`gettext_on_windows`)
- `selenium` 4.23.0+
- `sqlparse` 0.5.0+ (required)
- `tblib` 3.0.0+

You can find these dependencies in [pip requirements files](https://pip.pypa.io/en/latest/user_guide/#requirements-files) inside the `tests/requirements` directory of the Django source tree and install them like so:

<div class="console">

\$ python -m pip install -r tests/requirements/py3.txt

</div>

If you encounter an error during the installation, your system might be missing a dependency for one or more of the Python packages. Consult the failing package's documentation or search the web with the error message that you encounter.

You can also install the database adapter(s) of your choice using `oracle.txt`, `mysql.txt`, or `postgres.txt`.

If you want to test the memcached or Redis cache backends, you'll also need to define a `CACHES` setting that points at your memcached or Redis instance respectively.

To run the GeoDjango tests, you will need to `set up a spatial database
and install the Geospatial libraries</ref/contrib/gis/install/index>`.

Each of these dependencies is optional. If you're missing any of them, the associated tests will be skipped.

To run some of the autoreload tests, you'll need to install the [Watchman](https://facebook.github.io/watchman/) service.

### Code coverage

Contributors are encouraged to run coverage on the test suite to identify areas that need additional tests. The coverage tool installation and use is described in `testing code coverage<topics-testing-code-coverage>`.

To run coverage on the Django test suite using the standard test settings:

<div class="console">

\$ coverage run ./runtests.py --settings=test_sqlite

</div>

After running coverage, combine all coverage statistics by running:

<div class="console">

\$ coverage combine

</div>

After that generate the html report by running:

<div class="console">

\$ coverage html

</div>

When running coverage for the Django tests, the included `.coveragerc` settings file defines `coverage_html` as the output directory for the report and also excludes several directories not relevant to the results (test code or external code included in Django).

### Code coverage on pull requests

Django's continuous integration (CI) system automatically runs code coverage analysis on pull requests and posts a comment with a diff coverage report. This helps reviewers see which lines in the changed code are covered by tests.

**What the coverage report shows:**

The coverage report posted on pull requests uses [diff-cover](https://github.com/Bachmann1234/diff_cover) to analyze only the lines that were changed or added in the PR. It shows:

- Lines that are covered by tests (✓)
- Lines that are not covered by tests (✗)
- Lines that cannot be covered (e.g., comments, blank lines)

**Important limitations:**

When reviewing coverage reports on pull requests, keep these limitations in mind:

- **Database-specific code:** The CI coverage job runs tests using PostgreSQL on Ubuntu. Code paths specific to other databases (SQLite, MySQL, Oracle) will appear as "not covered" even if database-specific tests exist. This is expected and acceptable.
- **Platform-specific code:** Similarly, code that only runs on certain operating systems (Windows, macOS) will appear as not covered when run on Ubuntu.
- **Coverage doesn't equal quality:** A line being "covered" only means it was executed during tests. It doesn't guarantee the line is well-tested or that all edge cases are handled. During review, assess test quality beyond just coverage numbers.

Missing coverage should be considered a warning rather than a blocker and should be evaluated in context.

## Contrib apps

Tests for contrib apps can be found in the `tests/` directory, typically under `<app_name>_tests`. For example, tests for `contrib.auth` are located in `tests/auth_tests`.

## Troubleshooting

### Test suite hangs or shows failures on `main` branch

Ensure you have the latest point release of a `supported Python version
<faq-python-version-support>`, since there are often bugs in earlier versions that may cause the test suite to fail or hang.

On **macOS** (High Sierra and newer versions), you might see this message logged, after which the tests hang:

``` pytb
objc[42074]: +[__NSPlaceholderDate initialize] may have been in progress in
another thread when fork() was called.
```

To avoid this set a `OBJC_DISABLE_INITIALIZE_FORK_SAFETY` environment variable, for example:

``` shell
$ OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES ./runtests.py
```

Or add `export OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES` to your shell's startup file (e.g. `~/.profile`).

### Many test failures with `UnicodeEncodeError`

If the `locales` package is not installed, some tests will fail with a `UnicodeEncodeError`.

You can resolve this on Debian-based systems, for example, by running:

``` console
$ apt-get install locales
$ dpkg-reconfigure locales
```

You can resolve this for macOS systems by configuring your shell's locale:

``` console
$ export LANG="en_US.UTF-8"
$ export LC_ALL="en_US.UTF-8"
```

Run the `locale` command to confirm the change. Optionally, add those export commands to your shell's startup file (e.g. `~/.bashrc` for Bash) to avoid having to retype them.

### Tests that only fail in combination

In case a test passes when run in isolation but fails within the whole suite, we have some tools to help analyze the problem.

The `--bisect` option of `runtests.py` will run the failing test while halving the test set it is run together with on each iteration, often making it possible to identify a small number of tests that may be related to the failure.

For example, suppose that the failing test that works on its own is `ModelTest.test_eq`, then using:

<div class="console">

\$ ./runtests.py --bisect basic.tests.ModelTest.test_eq

</div>

will try to determine a test that interferes with the given one. First, the test is run with the first half of the test suite. If a failure occurs, the first half of the test suite is split in two groups and each group is then run with the specified test. If there is no failure with the first half of the test suite, the second half of the test suite is run with the specified test and split appropriately as described earlier. The process repeats until the set of failing tests is minimized.

The `--pair` option runs the given test alongside every other test from the suite, letting you check if another test has side-effects that cause the failure. So:

<div class="console">

\$ ./runtests.py --pair basic.tests.ModelTest.test_eq

</div>

will pair `test_eq` with every test label.

With both `--bisect` and `--pair`, if you already suspect which cases might be responsible for the failure, you may limit tests to be cross-analyzed by `specifying further test labels <runtests-specifying-labels>` after the first one:

<div class="console">

\$ ./runtests.py --pair basic.tests.ModelTest.test_eq queries transactions

</div>

You can also try running any set of tests in a random or reverse order using the `--shuffle` and `--reverse` options. This can help verify that executing tests in a different order does not cause any trouble:

<div class="console">

\$ ./runtests.py basic --shuffle \$ ./runtests.py basic --reverse

</div>

### Seeing the SQL queries run during a test

If you wish to examine the SQL being run in failing tests, you can turn on `SQL logging <django-db-logger>` using the `--debug-sql` option. If you combine this with `--verbosity=2`, all SQL queries will be output:

<div class="console">

\$ ./runtests.py basic --debug-sql

</div>

### Seeing the full traceback of a test failure

By default tests are run in parallel with one process per core. When the tests are run in parallel, however, you'll only see a truncated traceback for any test failures. You can adjust this behavior with the `--parallel` option:

<div class="console">

\$ ./runtests.py basic --parallel=1

</div>

You can also use the `DJANGO_TEST_PROCESSES` environment variable for this purpose.

## Tips for writing tests

### Isolating model registration

To avoid polluting the global `~django.apps.apps` registry and prevent unnecessary table creation, models defined in a test method should be bound to a temporary `Apps` instance. To do this, use the `~django.test.utils.isolate_apps` decorator:

    from django.db import models
    from django.test import SimpleTestCase
    from django.test.utils import isolate_apps


    class TestModelDefinition(SimpleTestCase):
        @isolate_apps("app_label")
        def test_model_definition(self):
            class TestModel(models.Model):
                pass

            ...

<div class="admonition">

Setting `app_label`

Models defined in a test method with no explicit `~django.db.models.Options.app_label` are automatically assigned the label of the app in which their test class is located.

In order to make sure the models defined within the context of `~django.test.utils.isolate_apps` instances are correctly installed, you should pass the set of targeted `app_label` as arguments:

``` python
from django.db import models
from django.test import SimpleTestCase
from django.test.utils import isolate_apps


class TestModelDefinition(SimpleTestCase):
    @isolate_apps("app_label", "other_app_label")
    def test_model_definition(self):
        # This model automatically receives app_label='app_label'
        class TestModel(models.Model):
            pass

        class OtherAppModel(models.Model):
            class Meta:
                app_label = "other_app_label"

        ...
```

</div>
