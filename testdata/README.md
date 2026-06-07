# Reference Fixtures

Each leaf directory is a focused OpenAPI input and one expected generated Elm
module. Feature folders group related standalone cases, so a feature can have
multiple minimal examples without combining variants into one schema.

The fixtures are intended to be portable examples that other implementations can
copy and use as a validation suite.

Fixture contract:

- `input.yaml` is the OpenAPI document.
- `module.txt` names the generated Elm module under test.
- `expected/` contains the exact expected Elm output for that module.
- `expected/` only includes schema modules generated for the fixture.

Keep each fixture focused on a single behavior. Add another leaf directory for a
feature variant unless the interaction between variants is the behavior under
test.

Refresh expected files after an intentional generator change with:

```sh
UPDATE_REFERENCE=1 go test ./...
```
