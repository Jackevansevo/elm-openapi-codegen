# elm-openapi-codegen

Generate Elm types and JSON decoders from an OpenAPI schema using Go.

The generator reads `components.schemas` from an OpenAPI document, normalizes the
schemas into an internal model, then writes Elm modules for the generated types
and decoders.

## Installation

Install the CLI with:

```sh
go install github.com/jackevansevo/elm-openapi-codegen/cmd/elm-openapi-codegen@latest
```

## Usage

```sh
elm-openapi-codegen --out <elm-output-dir> <openapi-spec-file>
```

For example:

```sh
elm-openapi-codegen --out src/Generated openapi.yaml
```

The output directory must be inside one of the `source-directories` from the
nearest `elm.json`. Elm module names are inferred from the output path relative
to that source directory. For example, with `"source-directories": ["src"]`,
`--out src/Generated` writes modules such as `Generated.User`.

# Example

```yaml
Book:
  type: object
  required:
    - author
    - bookId
    - bookTitle
  properties:
    author:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        website:
          type: string
          nullable: true
    bookId:
      type: string
    bookTitle:
      type: string
    inPrint:
      type: boolean
    tags:
      type: array
      items:
        type: string
```

Selected generated output:

```elm
module Generated.Book exposing (Book, bookDecoder)

import Generated.Author exposing (Author, authorDecoder)
import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline exposing (optional, required)


type alias Book =
    { author : Author
    , bookId : String
    , bookTitle : String
    , inPrint : Maybe Bool
    , tags : Maybe (List String)
    }


bookDecoder : Decoder Book
bookDecoder =
    Decode.succeed Book
        |> required "author" authorDecoder
        |> required "bookId" Decode.string
        |> required "bookTitle" Decode.string
        |> optional "inPrint" (Decode.map Just Decode.bool) Nothing
        |> optional "tags" (Decode.map Just (Decode.list Decode.string)) Nothing
```

```elm
module Generated.Author exposing (Author, authorDecoder)

import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline exposing (optional, required)


type alias Author =
    { name : String
    , website : Maybe String
    }


authorDecoder : Decoder Author
authorDecoder =
    Decode.succeed Author
        |> required "name" Decode.string
        |> optional "website" (Decode.nullable Decode.string) Nothing
```

Required fields use the mapped type directly. Optional fields are wrapped in
`Maybe` unless they have a supported default value. Nullable fields are also
wrapped in `Maybe` and decoded with `Decode.nullable`.

## Enum Types

String enums generate a custom union type with helper functions:

```yaml
BookStatus:
  type: string
  enum:
    - draft
    - published
    - archived
```

Generated output:

```elm
module Generated.BookStatus exposing (BookStatus(..), all, bookStatusDecoder, bookStatusEncoder, fromString, toString)

import Json.Decode as Decode exposing (Decoder)
import Json.Encode as Encode


type BookStatus
    = Draft
    | Published
    | Archived


all : List BookStatus
all =
    [ Draft
    , Published
    , Archived
    ]


toString : BookStatus -> String
toString value =
    case value of
        Draft ->
            "draft"

        Published ->
            "published"

        Archived ->
            "archived"


fromString : String -> Result String BookStatus
fromString value =
    case value of
        "draft" ->
            Ok Draft

        "published" ->
            Ok Published

        "archived" ->
            Ok Archived

        _ ->
            Err ("Unknown BookStatus: " ++ value)


bookStatusDecoder : Decoder BookStatus
bookStatusDecoder =
    Decode.string
        |> Decode.andThen
            (\value ->
                case fromString value of
                    Ok enumValue ->
                        Decode.succeed enumValue

                    Err err ->
                        Decode.fail err
            )


bookStatusEncoder : BookStatus -> Encode.Value
bookStatusEncoder value =
    Encode.string (toString value)
```

Integer, float, and boolean enums also generate `all`, `toString`, and `fromString` but keep type-specific
encoder and decoder implementations since their JSON wire format is not a string.

For more generated examples, see the [Examples wiki page](/Jackevansevo/elm-openapi-codegen/wiki/Examples).

# Implementation

By default openAPI schema fields are optional (not required)

```yaml
Category:
  type: object
  properties:
    id:
      type: integer
```

So a corresponding Elm type representation would use `Maybe`:

```elm
type alias Category =
    { id : Maybe Int
    }
```

Generated decoders trust the OpenAPI contract for present field values. Optional
fields use the fallback behavior from `Json.Decode.Pipeline.optional`:

```elm
type alias Category =
    { id : Maybe Int
    }


categoryDecoder : Decoder Category
categoryDecoder =
    Decode.succeed Category
        |> optional "id" (Decode.map Just Decode.int) Nothing
```

Behavior:

```elm
Decode.decodeString categoryDecoder "{}"
--> Ok { id = Nothing } -- ✅

Decode.decodeString categoryDecoder """{ "id": 1 }"""
--> Ok { id = Just 1 } -- ✅

Decode.decodeString categoryDecoder """{ "id": "old" }"""
--> Ok { id = Nothing } -- outside the OpenAPI contract
```

This keeps generated code small and follows the schema-as-contract model: a
server response with the wrong type is already outside the OpenAPI contract.

The generated decoders are not defensive validators for every possible JSON
shape. They assume the server returns data matching the OpenAPI schema. In
particular, optional fields with malformed present values may decode to their
fallback (`Nothing` or a supported default), just like missing fields. If you
need to detect contract violations such as `{ "id": "old" }` for an integer
field, validate responses separately or use stricter hand-written decoders for
that boundary.

## Tests

Run Go tests with:

```sh
go test ./...
```

The primary tests are top-level reference fixture tests. Each fixture in
[`testdata`](testdata) runs the generator and compares the
generated Elm module with the checked-in expected file.

Refresh expected files after an intentional generator change with:

```sh
UPDATE_REFERENCE=1 go test ./...
```

The test suite does not invoke Elm or depend on Elm tooling.
