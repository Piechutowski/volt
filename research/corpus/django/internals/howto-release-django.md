# How to release Django

This document explains how to release Django.

**Please, keep these instructions up-to-date if you make changes!** The point here is to be descriptive, not prescriptive, so feel free to streamline or otherwise make changes, but **update this document accordingly!**

## Overview

There are three types of releases that you might need to make:

- Security releases: disclosing and fixing a vulnerability. This'll generally involve two or three simultaneous releases -- e.g. 3.2.x, 4.0.x, and, depending on timing, perhaps a 4.1.x.
- Regular version releases: either a final release (e.g. 4.1) or a bugfix update (e.g. 4.1.1).
- Pre-releases: e.g. 4.2 alpha, beta, or rc.

The short version of the steps involved is:

1.  If this is a security release, pre-notify the security distribution list one week before the actual release.
2.  Proofread the release notes, looking for organization and writing errors. Draft a blog post and email announcement.
3.  Update version numbers and create the release artifacts.
4.  Create the new `Release` in the admin on `djangoproject.com`.
    1.  Set the proper date but ensure the flag `is_active` is disabled.
    2.  Upload the artifacts (tarball, wheel, and checksums).
5.  Verify package(s) signatures, check if they can be installed, and ensure minimal functionality.
6.  Upload the new version(s) to PyPI.
7.  Enable the `is_active` flag for each release in the admin on `djangoproject.com`.
8.  Post the blog entry and send out the email announcements.
9.  Update version numbers post-release in stable branch(es).
10. Add stub release notes for the next patch release in `main` and backport.

There are a lot of details, so please read on.

<div id="checklist-generator">

<div class="admonition">

Use the checklists app

To generate a checklist compiling the tasks described below as relevant to the specific release(s) you are issuing, use the checklists app in the [project admin](https://www.djangoproject.com/admin/checklists/). This populates a lot of boilerplate you will need for announcements, CVE publication, and hashes for commit messages. By using this app for preparing security issue metadata, your peer releasers can check your entries and consult them again in the future. See [example checklist](https://www.djangoproject.com/checklists/release/5.2.4/).

</div>

</div>

## Prerequisites

You'll need a few things before getting started. If this is your first release, you'll need to coordinate with another releaser to get all these things lined up, and write to the Ops mailing list requesting the required access and permissions.

- A Unix environment with these tools installed (in alphabetical order):

  - bash (version 4.0+)
  - git
  - GPG
  - make
  - man
  - hashing tools (typically `md5sum`, `sha1sum`, and `sha256sum` on Linux, or `md5` and `shasum` on macOS)
  - python

- A GPG key pair. Securely store the private part, and protect it with a passphrase. The public part needs to be uploaded to your GitHub account.

  <div class="admonition">

  More than one GPG key

  If the key you want to use is not your default signing key, you'll need to add `-u you@example.com` to every GPG signing command shown below, where `you@example.com` is the email address associated with the key you want to use.

  </div>

- A clean Python virtual environment (Python 3.10+, pip 26.1+) to build artifacts.

- Access to [Django's project on PyPI](https://pypi.org/project/Django/) to upload binaries, ideally with extra permissions to [yank a release](https://pypi.org/help/#yanked) if necessary. Ensure your PyPI account only uses WebAuthn-based authentication factors, not TOTP (one-time codes). Create an API token following the [official documentation](https://pypi.org/help/#apitoken), and set up your `$HOME/.pypirc` file like this:

  ``` ini
  [distutils]
    index-servers =
      pypi
      django

  [pypi]
    username = __token__
    password = # User-scoped or project-scoped token, to set as the default.

  [django]
    repository = https://upload.pypi.org/legacy/
    username = __token__
    password = # A project token.
  ```

- Access to [Django's project on Transifex](https://app.transifex.com/django/django/), with a Manager role. Generate an API Token in the [user setting section](https://app.transifex.com/user/settings/api/) and set up your `$HOME/.transifexrc` file like this:

  ``` ini
  [https://www.transifex.com]
    rest_hostname = https://rest.api.transifex.com
    token = # API token
  ```

- Access to the Django admin on `djangoproject.com` as a "Site maintainer".

- Access to create a post in the [Django Forum - Announcements category](https://forum.djangoproject.com/c/announcements/7) and to send emails to the [django-announce](https://groups.google.com/g/django-announce/) mailing list.

- Access to the `django-security` repo in GitHub. Among other things, this provides access to the pre-notification distribution list (needed for security release preparation tasks).

- Access to the Django project on [Read the Docs](https://readthedocs.org/projects/django/).

## Pre-release tasks

A few items need to be taken care of before even beginning the release process. This stuff starts about a week before the release; most of it can be done any time leading up to the actual release.

### 10 (or more) days before a security release

1.  Reserve one [CVE ID](https://www.cve.org/About/Overview/) per security issue as follows. (Or, if you lack CNA credentials, email `cna@djangoproject.com` with a request.)
    - Enable virtual environment with `cvelib` installed.

    - Export user information:

      ``` shell
      $ export CVE_USER=<user-email>@djangoproject.com CVE_ORG=DSF
      ```

    - Reserve:

      ``` shell
      $ cve --interactive reserve <quantity>
      ```
2.  Generate the relevant (private) patch(es) using `git format-patch`, one for the `main` branch and one for each stable branch being patched.

### A week before a security release

1.  Send out pre-notification exactly **one week** before the security release. The template for that email and a list of the recipients are in the private `django-security` GitHub wiki. BCC the pre-notification recipients, and be sure to include the relevant CVE IDs. Attach all the relevant patches (targeting `main` and the stable branches), and sign the email text with the key you'll use for the release, with a command like:

    ``` shell
    $ gpg --clearsign --digest-algo SHA256 prenotification-email.txt
    ```

2.  `Notify django-announce <security-disclosure>` of the upcoming security release with a general message such as:

    ``` text
    Notice of upcoming Django security releases (3.2.24, 4.2.10 and 5.0.2)

    Django versions 5.0.2, 4.2.10, and 3.2.24 will be released on Tuesday,
    February 6th, 2024 around 1500 UTC. They will fix one security defect
    with severity "moderate".

    For details of severity levels, see:
    https://docs.djangoproject.com/en/dev/internals/security/#how-django-discloses-security-issues
    ```

3.  Prepare issue metadata:

    - Severity
    - Short description
    - Reporter
    - Remediator
    - Reported at
    - Confirmed at (usually date CVE reserved)
    - CWE Problem Type
    - CAPEC Impact Type
    - CVSS (4.0) Score & Vector

### A few days before any release

1.  As the release approaches, watch Trac to make sure no release blockers are left for the upcoming release. Under exceptional circumstances, such as to meet a pre-determined security release date, a release could still go ahead with an open release blocker. The releaser is trusted with the decision to release with an open release blocker or to postpone the release date of a non-security release if required.

2.  Check with the other mergers to make sure they don't have any uncommitted changes for the release.

3.  Proofread the release notes, including looking at the online version to `catch any broken links <documentation-link-check>` or reST errors, and make sure the release notes contain the correct date.

4.  Double-check that the release notes mention deprecation timelines for any APIs noted as deprecated, and that they mention any changes in Python version support.

5.  Double-check that the release notes index has a link to the notes for the new release; this will be in `docs/releases/index.txt`.

6.  If this is a `feature release`, ensure translations from Transifex have been integrated.

    In addition to having a configured Transifex account, ensure that the [tx CLI](https://developers.transifex.com/docs/cli) is available in your `PATH`. You can then fetch all translations since a given date by running:

    ``` shell
    $ python scripts/manage_translations.py fetch -v 1 --since=<some date>
    ```

    To determine a good value for `--since`, check the date of the most recent commit with wording similar to `Updated translations from Transifex` and use a date a few days prior.

    This command takes some time to run. When done, carefully inspect the output for potential errors and/or warnings. If there are some, you will need to debug and resolve them on a case by case basis.

    The recently fetched translations need some manual adjusting. First of all, the `PO-Revision-Date` values must be manually bumped to be later than `POT-Creation-Date`. You can use a command similar to this to bulk update all the `.po` files (compare the diff against the relevant stable branch):

    ``` shell
    $ git diff --name-only stable/5.0.x | grep "\.po"  | xargs sed -ri "s/PO-Revision-Date: [0-9\-]+ /PO-Revision-Date: $(date -I) /g"
    ```

    Lastly, commit the changed/added files (both `.po` and `.mo`) and create a new PR targeting the stable branch of the corresponding release (example [PR updating translations for 4.2](https://github.com/django/django/pull/16715)).

    Once merged, forward port the changes into `main` (`example commit
    <cb27e5b9c0703fb0edd70b2138e3e53a78c9551d>`).

7.  `Update the django-admin manual page <django-admin-manpage>`:

    ``` shell
    $ cd docs
    $ make man
    $ man _build/man/django-admin.1  # do a quick sanity check
    $ cp _build/man/django-admin.1 man/django-admin.1
    ```

    and then commit the changed man page.

8.  If this is the "dot zero" release of a new series, create a new branch from the current stable branch in the [django-docs-translations](https://github.com/django/django-docs-translations) repository. For example, when releasing Django 4.2:

    ``` shell
    $ git checkout -b stable/4.2.x origin/stable/4.1.x
    $ git push origin stable/4.2.x:stable/4.2.x
    ```

9.  Write the announcement blog post for the release. You can enter it into the admin at any time and mark it as inactive. Here are a few examples: [example security release announcement](https://www.djangoproject.com/weblog/2013/feb/19/security/), [example regular release announcement](https://www.djangoproject.com/weblog/2012/mar/23/14/), [example pre-release announcement](https://www.djangoproject.com/weblog/2012/nov/27/15-beta-1/).

#### A few days before a feature freeze

In preparation for the alpha release, the directory `/home/www/www/media/releases/A.B` must be created on the djangoproject server.

Before the feature freeze, a branch targeting `main` must be created to prepare for the next feature release. It should be reviewed and approved a few days before the freeze, allowing it to be merged after the stable branch is cut. The following items should be addressed in this branch:

1.  Update the `VERSION` tuple in `django/__init__.py`, incrementing to the next expected release (`example commit
    <96700c7b378c592f0b1732302c22af2fd2c87fc6>`).
2.  Create a stub release note for the next feature release. Use the stub from the previous feature release or copy the contents from the current version and delete most of the contents leaving only the headings (`example commit <9b5ad4056ccf9ff7ea548f72d28eb66c1b4f84cc>`).
3.  Remove `.. versionadded::` and `.. versionchanged::` annotations in the documentation from two releases ago, as well as any remaining older annotations. For example, in Django 5.1, notes for 4.2 will be removed (`example commit <9edb7833b89e811eefd94974fb987f4605b0c0d7>`).
4.  Remove features that have reached the end of their deprecation cycle, including their docs and the `.. deprecated::` annotation. Each removal should be done in a separate commit for clarity. In the commit message, add a `Refs #XXXXX --` prefix linking to the original ticket where the deprecation began if possible. Make sure this gets noted in the removed features section in the release notes (`example commit
    <f2d9c76aa7096ef3eed675b9eb824858f9dd81e5>`).
5.  Advance the deprecation warnings (`example commit
    <0c0bda7f79a2de89a55cfcc8e60467d6588f406f>`).
6.  Increase the default PBKDF2 iterations in `django.contrib.auth.hashers.PBKDF2PasswordHasher` by about 20% (pick a round number). Run the tests, and update the 3 failing hasher tests with the new values. Make sure this gets noted in the release notes (`example commit
    <7288866da4dddf3705148c703421858ec19cdb78>`).

Concrete examples for past feature release bootstrap branches: [5.2 bootstrap](https://github.com/django/django/pull/18127), [5.1 bootstrap](https://github.com/django/django/pull/17246), [5.0 bootstrap](https://github.com/django/django/pull/16432).

## Feature freeze tasks

1.  Remove empty sections from the release notes (`example commit
    <9e6e58bad237a80ddd5e3ab8b834cecdaad8455e>`).

2.  Build the release notes locally and read them. Make any necessary change to improve flow or fix grammar (`example commit
    <435bdab93889dae01e71c79598edab10627cc1f9>`).

3.  Create a new stable branch from `main`. Be sure to fetch and update `upstream` to latest. For example, when feature freezing Django 5.2:

    ``` shell
    $ git fetch upstream
    $ git checkout -b stable/5.2.x upstream/main
    $ git push upstream -u stable/5.2.x:stable/5.2.x
    ```

    At the same time, update the `django_next_version` variable in `docs/conf.py` on the stable release branch to point to the new development version. For example, when creating `stable/5.2.x`, set `django_next_version` to `'6.0'` on the new stable branch (`example commit <1eb62e5b622ef7fd6e0123d8bbf6662d893d5d08>`).

4.  Create `Release` entries for the next version in the <span class="title-ref">admin</span>\_\_ on `djangoproject.com`. Add one for each milestone (alpha, beta, rc, and final), leaving *is active* unset to mark them as unreleased. Set target dates per the agreed schedule, and set the LTS flag if applicable. The `X.Y` roadmap page will be available at `/download/X.Y/roadmap/`.

    For example, when creating `stable/5.2.x`, add `Release` entries for `6.0a1`, `6.0b1`, `6.0rc1`, and `6.0`. The `6.0` roadmap can be then reviewed at <https://www.djangoproject.com/download/6.0/roadmap/>.

    \_\_ <https://www.djangoproject.com/admin/releases/release/add/>

5.  Go to the <span class="title-ref">Add document release page in the admin</span>\_\_, create a new `DocumentRelease` object for the English language for the newly created `Release` object. Do not mark this as default.

    \_\_ <https://www.djangoproject.com/admin/docs/documentrelease/add/>

6.  Add the new branch to [Read the Docs](https://readthedocs.org/projects/django/). Since the automatically generated version names ("stable-A.B.x") differ from the version names used in Read the Docs ("A.B.x"), update the Read the Docs config for the version to point to the slug `A.B.x` and set it as active. [See more details](https://github.com/readthedocs/readthedocs.org/issues/12483).

7.  [Create a PR on PyPI proposing the new Trove classifier](https://github.com/pypa/trove-classifiers/pulls?q=is%3Apr+django+trove+classifier). For example `Framework :: Django :: 5.2`.

8.  Update the current branch under active development and add pre-release branch in the [Django release process](https://code.djangoproject.com/#Djangoreleaseprocess) on Trac.

9.  Update the `docs/fixtures/doc_releases.json` JSON fixture for djangoproject.com, so people without access to the production DB can still run an up-to-date copy of the docs site ([example PR](https://github.com/django/djangoproject.com/pull/1446)). This will be merged after the final release.

## Actually rolling the release

OK, this is the fun part, where we actually push out a release! If you're issuing **multiple releases**, repeat these steps for each release.

1.  Check <span class="title-ref">Jenkins</span>\_\_ is green for the version(s) you're putting out. You probably shouldn't issue a release until it's green, and you should make sure that the latest green run includes the changes that you are releasing.

    \_\_ <https://djangoci.com>

2.  Cleanup the release notes for this release. Make these changes in `main` and backport to all branches where the release notes for a particular version are located.

    1.  For a feature release, remove the `UNDER DEVELOPMENT` header at the top of the release notes, remove the `Expected` prefix and update the release date, if necessary (`example commit
        <1994a2643881a9e3f9fa8d3e0794c1a9933a1831>`).
    2.  For a patch release, remove the `Expected` prefix and update the release date for all releases, if necessary (`example commit
        <34a503162fe222033a1cd3249bccad014fcd1d20>`).

3.  Regenerate a fresh, dedicated virtual environment for the release tools using a cooldown:

    ``` shell
    $ python -m pip install build twine --uploaded-prior-to=P7D
    ```

4.  A release always begins from a release branch, so you should make sure you're on an up-to-date stable branch. For example:

    ``` shell
    $ git checkout stable/4.1.x
    $ git pull
    ```

5.  If this is a security release, merge the appropriate patches from `django-security`. Rebase these patches as necessary to make each one a plain commit on the release branch rather than a merge commit. To ensure this, merge them with the `--ff-only` flag; for example:

    ``` shell
    $ git checkout stable/4.1.x
    $ git merge --ff-only security/4.1.x
    ```

    (This assumes `security/4.1.x` is a branch in the `django-security` repo containing the necessary security patches for the next release in the 4.1 series.)

    If git refuses to merge with `--ff-only`, switch to the security-patch branch and rebase it on the branch you are about to merge it into (`git checkout security/4.1.x; git rebase stable/4.1.x`) and then switch back and do the merge. Make sure the commit message for each security fix explains that the commit is a security fix and that an announcement will follow (`example security commit <bf39978a53f117ca02e9a0c78b76664a41a54745>`).

6.  Update the version number in `django/__init__.py` for the release. Please see [notes on setting the VERSION tuple](#notes-on-setting-the-version-tuple) below for details on `VERSION` (`example commit
    <2719a7f8c161233f45d34b624a9df9392c86cc1b>`).

    1.  If this is a pre-release package also update the "Development Status" trove classifier in `pyproject.toml` to reflect this. An `rc` pre-release should not change the trove classifier (`example
        commit for alpha release <759921c8e9ad151932fc913ab429fef0a6112ef8>`, `example commit for beta release
        <25fec8940b24107e21314ab6616e18ce8dec1c1c>`).
    2.  Otherwise, make sure the classifier is set to `Development Status :: 5 - Production/Stable`.

### Building the artifacts

<div class="admonition">

Optionally use helper scripts

You can streamline some of the steps below using helper scripts from the `scripts` folder:

- Release script example run:

``` shell
$ PGP_KEY_ID=<key-id> PGP_KEY_URL=<key-url> DEST_FOLDER=~/releases scripts/do_django_release.py
```

- Verify the release (after artifacts were uploaded):

``` shell
$ VERSION=5.2.1 scripts/verify_release.sh
```

</div>

1.  Tag the release using `git tag`. For example:

    ``` shell
    $ git tag --sign --message="Tag 4.1.1" 4.1.1
    ```

    You can check your work running `git tag --verify <tag>`.

2.  Make sure you have an absolutely clean tree by running `git clean -dfx`.

3.  Run `python -m build` to generate the release packages. This will create the release artifacts (tarball and wheel) in a `dist/` directory.

4.  Generate the hashes of the release packages:

    ``` shell
    $ cd dist
    $ md5sum *
    $ sha1sum *
    $ sha256sum *
    ```

5.  Create a "checksums" file, `Django-<<VERSION>>.checksum.txt` containing the hashes and release information. Start with this template and insert the correct version, date, GPG key ID (from `gpg --list-keys --keyid-format LONG`), release manager's GitHub username, release URL, and checksums:

    ``` text
    This file contains MD5, SHA1, and SHA256 checksums for the source-code
    tarball and wheel files of Django <<VERSION>>, released <<DATE>>.

    To use this file, you will need a working install of PGP or other
    compatible public-key encryption software. You will also need to have
    the Django release manager's public key in your keyring. This key has
    the ID ``XXXXXXXXXXXXXXXX`` and can be imported from GitHub, for example,
    if using the open-source GNU Privacy Guard implementation of PGP:

        curl https://github.com/<<RELEASE MANAGER GITHUB USERNAME>>.gpg | gpg --import -

    Once the key is imported, verify this file:

        gpg --verify <<THIS FILENAME>>

    Once you have verified this file, you can use normal MD5, SHA1, or SHA256
    checksumming applications to generate the checksums of the Django
    package and compare them to the checksums listed below.

    Release packages
    ================

    https://www.djangoproject.com/download/<<VERSION>>/tarball/
    https://www.djangoproject.com/download/<<VERSION>>/wheel/

    MD5 checksums
    =============

    <<MD5SUM>>  <<RELEASE TAR.GZ FILENAME>>
    <<MD5SUM>>  <<RELEASE WHL FILENAME>>

    SHA1 checksums
    ==============

    <<SHA1SUM>>  <<RELEASE TAR.GZ FILENAME>>
    <<SHA1SUM>>  <<RELEASE WHL FILENAME>>

    SHA256 checksums
    ================

    <<SHA256SUM>>  <<RELEASE TAR.GZ FILENAME>>
    <<SHA256SUM>>  <<RELEASE WHL FILENAME>>
    ```

6.  Sign the checksum file (`gpg --clearsign --digest-algo SHA256 Django-<version>.checksum.txt`). This generates a signed document, `Django-<version>.checksum.txt.asc` which you can then verify using `gpg --verify Django-<version>.checksum.txt.asc`.

## Making the release(s) available to the public

Now you're ready to actually put the release out there. To do this:

1.  Create a new `Release` entry in the [djangoproject.com's admin](https://www.djangoproject.com/admin/releases/release/add/). If this is a security release, this should be done 15 minutes before the announced release time, no sooner:

    Version  
    Must match the version number as defined in the tarball (`django-<version>.tar.gz`). For example: "5.2", "4.1.1", or "4.2rc1".

    Is active  
    Set to False until the release is fully published (last step).

    LTS  
    Enable if the release is part of an `LTS (Long Term Support)` branch.

    Dates  
    Set the release date to today. This release will not be published until `is_active` is enabled.

    Artifacts  
    Upload the tarball (`django-<version>.tar.gz`), wheel (`django-<version>-py3-none-any.whl`), and checksum (`django-<version>.checksum.txt.asc`) files created earlier.

2.  Test that the release packages install correctly using `pip`. Here's one simple method (this just tests that the binaries are available, that they install correctly, and that migrations and the development server start, but it'll catch silly mistakes): <https://code.djangoproject.com/wiki/ReleaseTestNewVersion>.

3.  Run the <span class="title-ref">confirm-release</span>\_\_ build on Jenkins to verify the checksum file(s) (e.g. use `4.2rc1` for <https://media.djangoproject.com/pgp/Django-4.2rc1.checksum.txt>).

    \_\_ <https://djangoci.com/job/confirm-release/>

4.  Generate a new token for this release on PyPI, and store it in `.pypirc` as shown above.

5.  Upload the release packages to PyPI:

    ``` shell
    $ twine upload --repository django dist/*
    ```

6.  On PyPI, revoke the token you just created.

7.  Update the newly created `Release` in the admin in `djangoproject.com` and enable the `is_active` flag.

8.  Push your work and the new tag:

    ``` shell
    $ git push
    $ git push --tags
    ```

9.  Make the blog post announcing the release live.

10. For a new version release (e.g. 4.1, 4.2), update the default stable version of the docs by flipping the `is_default` flag to `True` on the appropriate `DocumentRelease` object in the `docs.djangoproject.com` database (this will automatically flip it to `False` for all others); you can do this using the site's admin.

    Create new `DocumentRelease` objects for each language that has an entry for the previous release. Update djangoproject.com's <span class="title-ref">robots.docs.txt</span>\_\_ file by copying the result generated from running the command `manage_translations.py robots_txt` in the current stable branch from the <span class="title-ref">django-docs-translations repository</span>\_\_. For example, when releasing Django 4.2:

    ``` shell
    $ git checkout stable/4.2.x
    $ git pull
    $ python manage_translations.py robots_txt
    ```

    \_\_ <https://github.com/django/djangoproject.com/blob/main/djangoproject/static/robots.docs.txt> \_\_ <https://github.com/django/django-docs-translations>

11. Post the release announcement to the mailing list and the Django Forum. This should include a link to the announcement blog post.

12. If this is a security release, publish the CVE metadata. (The `checklist app<checklist-generator>` generates JSON for this.):

    ``` shell
    $ cve publish <cve-number> --cve-json-file <path-to-file>
    ```

13. If this is a security release, send a separate email to `oss-security@lists.openwall.com`. Provide "Django" plus the CVE IDs in the subject line. The message body should include the vulnerability details, for example, the announcement blog post text. Include a link to the announcement blog post.

## Post-release

You're almost done! All that's left to do now is:

1.  If this is not a pre-release, update the `VERSION` tuple in `django/__init__.py` again, incrementing to whatever the next expected release will be. For example, after releasing 4.1.1, update `VERSION` to `VERSION = (4, 1, 2, 'alpha', 0)` (`example commit
    <a4d19953d46247ee1992b3427fe652e941524272>`).
2.  If this was an alpha release:
    1.  Add the feature release version in [Trac's versions list](https://code.djangoproject.com/admin/ticket/versions).

    2.  Create a new security branch from the freshly cut stable branch. Be sure to fetch and update `upstream` to latest. For example, after the 5.2 alpha release:

        ``` shell
        $ git fetch upstream
        $ git checkout -b security/5.2.x upstream/stable/5.2.x
        $ git push origin -u security/5.2.x:security/5.2.x
        ```
3.  If this was a final release:
    1.  Update the `default_version` setting in the code.djangoproject.com's `trac.ini` file ([example PR](https://github.com/django/code.djangoproject.com/pull/268)).
    2.  Update the current stable branch and remove the pre-release branch in the [Django release process](https://code.djangoproject.com/#Djangoreleaseprocess) on Trac.
    3.  Update djangoproject.com's download page ([example PR](https://github.com/django/djangoproject.com/pull/1444)).
    4.  Process the older versions that will reach End-Of-Mainstream and/or End-Of-Life support when this final release is published:
        1.  Ensure that the EOL versions are mentioned in the blog post. See [example announcement](https://www.djangoproject.com/weblog/2025/apr/02/django-52-released/).
        2.  Create a tag for the EOL stable branch and delete the stable branch. Inspect and use the `scripts/archive_eol_stable_branches.py` helper.
4.  If this was a security release, update `/releases/security` with details of the issues addressed.
5.  If this was a pre-release, the translation catalogs need to be updated:
    1.  Make a new branch from the recently released stable branch:

        ``` shell
        $ git checkout stable/A.B.x
        $ git checkout -b update-translations-catalog-A.B.x
        ```

    2.  Ensure that the release's dedicated virtual environment is enabled and run the following:

        ``` shell
        $ cd django
        $ django-admin makemessages -l en --domain=django
        processing locale en
        $ django-admin makemessages -l en --domain=djangojs
        processing locale en
        ```

    3.  Make a pull request against the corresponding stable branch and merge once approved.

    4.  Forward port the updated source translations to the `main` branch (`example commit <aed303aff57ac990894b6354af001b0e8ea55f71>`).
6.  If this was an `alpha` pre-release, coordinate with the maintainer of `asgiref` to determine when to raise the minimum version before the final release. For the backport, add an upper bound on the next major version (example commits: `main <df35cf578f99522dd1ba864d513be95d47bab7a5>`, `backport <e5d2664908164d51d2daa46e375da8b6a93def03>`).
7.  If this was an `rc` pre-release, call for translations for the upcoming release in the [Django Forum - Internationalization category](https://forum.djangoproject.com/c/internals/i18n/14).

## Notes on setting the VERSION tuple

Django's version reporting is controlled by the `VERSION` tuple in `django/__init__.py`. This is a five-element tuple, whose elements are:

1.  Major version.
2.  Minor version.
3.  Micro version.
4.  Status -- can be one of "alpha", "beta", "rc" or "final".
5.  Series number, for alpha/beta/RC packages which run in sequence (allowing, for example, "beta 1", "beta 2", etc.).

For a final release, the status is always "final" and the series number is always 0. A series number of 0 with an "alpha" status will be reported as "pre-alpha".

Some examples:

- `(4, 1, 1, "final", 0)` → "4.1.1"
- `(4, 2, 0, "alpha", 0)` → "4.2 pre-alpha"
- `(4, 2, 0, "beta", 1)` → "4.2 beta 1"
