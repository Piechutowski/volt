# Accessibility

The Django project is committed to ensuring that websites built with Django are usable by everyone, including people with disabilities. Django's built-in components, such as the admin interface and default form rendering, should adhere to established accessibility standards and meet our own targets for supporting specific input devices and assistive technologies.

## Accessibility standards

We work to conform with the [Web Content Accessibility Guidelines](https://www.w3.org/TR/WCAG22/) (WCAG), version 2.2, at the AA level. WCAG is the most established standard for web accessibility. [AA-level guidelines](https://www.w3.org/WAI/WCAG22/quickref/?currentsidebar=%23col_overview&levels=aaa) are the most common legal compliance target worldwide.

We also aim to follow other best practices, such as:

- WCAG 2.2 [AAA-level guidelines](https://www.w3.org/WAI/WCAG22/quickref/?currentsidebar=%23col_overview&levels=a%2Caa) – stricter criteria that go beyond the AA level.
- The upcoming [WCAG 3.0](https://www.w3.org/TR/wcag-3.0/) guidelines – a new, evolving standard that aims to unify and improve upon previous versions of WCAG.
- [Authoring Tool Accessibility Guidelines](https://www.w3.org/TR/ATAG20/) (ATAG) 2.0 – guidelines for software and services that are used to produce web content.

To learn more about accessibility without diving straight into standards, we recommend [The A11Y Project](https://www.a11yproject.com/), a community-driven effort to make digital accessibility easier.

## Support targets and testing

Beyond standards, we want to make sure Django actually works for people using a wide range of input devices and assistive technologies. This has benefits to make sure Django works better [for everyone](https://en.wikipedia.org/wiki/Curb_cut_effect). The best way to do this effectively is to take accessibility considerations into account as part of designing features. If in doubt, consult with users who rely on assistive technologies or with accessibility experts. You can reach out to the [Accessibility team](https://www.djangoproject.com/foundation/teams/#accessibility-team) via the [Accessibility Django forum topic](https://forum.djangoproject.com/c/internals/accessibility/26) or via the `#accessibility` channel on the [Django Discord server](https://chat.djangoproject.com).

### Testing baseline

Design the UI with accessibility in mind, and the testing will only be needed as a final check. For more complex interfaces, confer with other contributors to decide on testing targets. Reach out to the [Accessibility team](https://www.djangoproject.com/foundation/teams/#accessibility-team) for support and to coordinate testing.

Always test user interface changes with:

- Keyboard-only navigation. Common issues include:
  - An interactive element can't be reached using the Tab or arrow keys.
  - An interactive element "traps" input focus and prevents navigating away.
  - An interactive element doesn't give a visible indication when it has input focus.
  - The focus order is inconsistent with the logical order that's communicated visually.
- The [Accessibility Insights](https://accessibilityinsights.io/) browser extension's automated checks feature, or an equivalent tool with the [Axe](https://github.com/dequelabs/axe-core) checker.

Where the UI changes could affect those modalities, also test with:

- Touch-only navigation. Common issues include:
  - A touch target (interactive element) is too small.
  - Hover-based interaction that does not translate to touch, such as a hover-only tooltip.
- 400% browser zoom. Common issues include:
  - Content is cut off or disappears when zoomed.
  - Content that does not inherently require a two-dimensional layout causes scrolling in both directions (vertical and horizontal). Two-dimensional scrolling is acceptable for content like images, maps, videos, and data tables, which require both dimensions to be usable.
- Forced-colors mode (for example Windows Contrast Themes). Note that this can be tested via the `"high_contrast"` mode in the `screenshot tests
  <screenshot-tests>`. Common issues include:
  - Over-reliance on color for meaning, which is lost in forced-colors mode.
  - Use of `!important` or inline styles may break forced-colors mode.

### Recommended assistive technologies

Where the UI changes could affect assistive technologies, here are popular free options we recommend testing with.

For Windows:

- [NVDA](https://www.nvaccess.org/) - recommended with Firefox ESR
- [Narrator](https://support.microsoft.com/en-us/windows/complete-guide-to-narrator-e4397a0d-ef4f-b386-d8ae-c172f109bdb1) - recommended with Microsoft Edge
- Windows Magnifier
- Windows Speech Recognition
- [Contrast themes](https://support.microsoft.com/en-us/windows/change-color-contrast-in-windows-fedc744c-90ac-69df-aed5-c8a90125e696)

For macOS:

- [VoiceOver](https://support.apple.com/en-gb/guide/voiceover-guide/welcome/web) - recommended with Safari
- macOS Zoom
- macOS Voice Control

For Linux:

- [Orca](https://help.gnome.org/users/orca/stable/) - recommended with Firefox ESR

For mobile or tablet:

- VoiceOver on iOS, or TalkBack on Android

The following are popular licensed options. If you are a user of these, or can work with a user holding a license, also test against:

- [JAWS](https://www.freedomscientific.com/products/software/jaws/) on Windows
- [Dragon](https://www.nuance.com/en-gb/dragon.html) on Windows

## Known issues and how to help

There are parts of Django that do not meet our accessibility targets. We actively work on fixing issues, both as part of ongoing maintenance and bigger overhauls. To learn about known issues, and get involved, see:

- `#accessibility` on the [Django Discord server](https://chat.djangoproject.com).
- The [Accessibility Django forum topic](https://forum.djangoproject.com/c/internals/accessibility/26).
- [Accessibility issues on the ticket tracker](https://code.djangoproject.com/query?status=!closed&keywords=~accessibility).
- Our [django accessibility improvements](https://github.com/orgs/django/projects/7) project board.
- The [Accessibility team](https://www.djangoproject.com/foundation/teams/#accessibility-team).
