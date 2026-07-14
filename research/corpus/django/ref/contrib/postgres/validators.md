# Validators

<div class="module">

django.contrib.postgres.validators

</div>

These validators are available from the `django.contrib.postgres.validators` module.

## `KeysValidator`

<div class="KeysValidator(keys, strict=False, messages=None)">

Validates that the given keys are contained in the value. If `strict` is `True`, then it also checks that there are no other keys present.

The `messages` passed should be a dict containing the keys `missing_keys` and/or `extra_keys`.

<div class="note">

<div class="title">

Note

</div>

Note that this checks only for the existence of a given key, not that the value of a key is non-empty.

</div>

</div>

## Range validators

### `RangeMaxValueValidator`

<div class="RangeMaxValueValidator(limit_value, message=None)">

Validates that the upper bound of the range is not greater than `limit_value`.

</div>

### `RangeMinValueValidator`

<div class="RangeMinValueValidator(limit_value, message=None)">

Validates that the lower bound of the range is not less than the `limit_value`.

</div>
