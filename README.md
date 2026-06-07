# elm-openapi-codegen

Generate Elm types and JSON decoders from an OpenAPI schema using Go.

The generator reads `components.schemas` from an OpenAPI document, normalizes the
schemas into an internal model, then writes Elm modules for the generated types
and decoders.

## Usage

Install the CLI with:

```sh
go install github.com/jackevansevo/elm-openapi-codegen/cmd/elm-openapi-codegen@latest
```

## Elm Dependencies

Generated decoders use `Json.Decode.Pipeline` from
`NoRedInk/elm-json-decode-pipeline`. Install it in the Elm project that will
compile the generated modules:

```sh
elm install NoRedInk/elm-json-decode-pipeline
```

```sh
elm-openapi-codegen --out <module-root-dir> [--module-root Generated] <openapi-spec-file>
```

`--out` is required. It is the exact directory that corresponds to the Elm
module root; the generator does not append the module root for you. With the
default `--module-root Generated`, pass a directory such as `src/Generated`.

For example:

```sh
elm-openapi-codegen --out src/Generated --module-root Generated testdata/object/primitive-field/input.yaml
```

That writes files such as `src/Generated/User.elm` while the Elm module remains
`Generated.User`.

For a custom module root, point `--out` at the matching directory:

```sh
elm-openapi-codegen --out src/Api/V1 --module-root Api.V1 openapi.yaml
```

That writes files such as `src/Api/V1/User.elm` while the Elm module remains
`Api.V1.User`.

# Examples

The YAML snippets below are `components.schemas` fragments. Full OpenAPI
documents for these examples live in [`testdata/examples`](testdata/examples)
and are checked by the test suite.

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

## Advanced Examples

### Enum

```yaml
BookStatus:
  type: string
  enum:
    - draft
    - published
    - archived
```

```elm
module Generated.BookStatus exposing (BookStatus(..), bookStatusDecoder, bookStatusEncoder)

import Json.Decode as Decode exposing (Decoder)
import Json.Encode as Encode


type BookStatus
    = Draft
    | Published
    | Archived


bookStatusDecoder : Decoder BookStatus
bookStatusDecoder =
    Decode.oneOf
        [ Decode.map (always Draft) (exactString "draft")
        , Decode.map (always Published) (exactString "published")
        , Decode.map (always Archived) (exactString "archived")
        ]


bookStatusEncoder : BookStatus -> Encode.Value
bookStatusEncoder value =
    case value of
        Draft ->
            Encode.string "draft"

        Published ->
            Encode.string "published"

        Archived ->
            Encode.string "archived"


exactString : String -> Decoder String
exactString expected =
    Decode.string
        |> Decode.andThen
            (\actual ->
                if actual == expected then
                    Decode.succeed actual

                else
                    Decode.fail "Unexpected value"
            )
```

### Default

```yaml
InventoryItem:
  type: object
  properties:
    stock:
      type: integer
      default: 0
```

```elm
module Generated.InventoryItem exposing (InventoryItem, inventoryItemDecoder)

import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline exposing (optional)


type alias InventoryItem =
    { stock : Int
    }


inventoryItemDecoder : Decoder InventoryItem
inventoryItemDecoder =
    Decode.succeed InventoryItem
        |> optional "stock" Decode.int 0
```

### `anyOf`

```yaml
Animal:
  anyOf:
    - $ref: '#/components/schemas/Cat'
    - $ref: '#/components/schemas/Dog'
```

```elm
module Generated.Animal exposing (Animal(..), animalDecoder)

import Generated.Cat exposing (Cat, catDecoder)
import Generated.Dog exposing (Dog, dogDecoder)
import Json.Decode as Decode exposing (Decoder)


type Animal
    = AnimalCat Cat
    | AnimalDog Dog


animalDecoder : Decoder Animal
animalDecoder =
    Decode.oneOf
        [ Decode.map AnimalCat catDecoder
        , Decode.map AnimalDog dogDecoder
        ]
```

### Discriminator

```yaml
Pet:
  oneOf:
    - $ref: '#/components/schemas/Cat'
    - $ref: '#/components/schemas/Dog'
  discriminator:
    propertyName: petType
    mapping:
      cat: '#/components/schemas/Cat'
      dog: '#/components/schemas/Dog'
```

```elm
module Generated.Pet exposing (Pet(..), petDecoder)

import Generated.Cat exposing (Cat, catDecoder)
import Generated.Dog exposing (Dog, dogDecoder)
import Json.Decode as Decode exposing (Decoder)


type Pet
    = PetCat Cat
    | PetDog Dog


petDecoder : Decoder Pet
petDecoder =
    Decode.field "petType" Decode.string
        |> Decode.andThen
            (\value ->
                case value of
                    "cat" ->
                        Decode.map PetCat catDecoder

                    "dog" ->
                        Decode.map PetDog dogDecoder

                    other ->
                        Decode.fail ("Unknown Pet: " ++ other)
            )
```

Variant schemas use exact string decoders for the discriminator field:

```elm
type alias Cat =
    { livesLeft : Maybe Int
    , name : String
    , petType : String
    }


catDecoder : Decoder Cat
catDecoder =
    object
        (Decode.succeed Cat
            |> optional "livesLeft" (Decode.map Just Decode.int) Nothing
            |> required "name" Decode.string
            |> required "petType" (exactString "cat")
        )
```

### `allOf`

```yaml
Book:
  allOf:
    - $ref: '#/components/schemas/CatalogItem'
    - type: object
      properties:
        authorName:
          type: string
        isbn:
          type: string

CatalogItem:
  type: object
  properties:
    itemId:
      type: string
    title:
      type: string
```

```elm
module Generated.Book exposing (Book, bookDecoder)

import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline exposing (optional)


type alias Book =
    { itemId : Maybe String
    , title : Maybe String
    , authorName : Maybe String
    , isbn : Maybe String
    }


bookDecoder : Decoder Book
bookDecoder =
    Decode.succeed Book
        |> optional "itemId" (Decode.map Just Decode.string) Nothing
        |> optional "title" (Decode.map Just Decode.string) Nothing
        |> optional "authorName" (Decode.map Just Decode.string) Nothing
        |> optional "isbn" (Decode.map Just Decode.string) Nothing
```

### Recursion

```yaml
Comment:
  type: object
  required:
    - author
    - body
  properties:
    author:
      type: string
    body:
      type: string
    replies:
      type: array
      items:
        $ref: '#/components/schemas/Comment'
```

```elm
module Generated.Comment exposing (Comment, commentDecoder)

import Json.Decode as Decode exposing (Decoder)
import Json.Decode.Pipeline exposing (optional, required)


type Comment
    = Comment
        { author : String
        , body : String
        , replies : Maybe (List Comment)
        }


commentDecoder : Decoder Comment
commentDecoder =
    Decode.lazy
        (\_ ->
            Decode.succeed
                (\author body replies ->
                    Comment
                        { author = author
                        , body = body
                        , replies = replies
                        }
                )
                |> required "author" Decode.string
                |> required "body" Decode.string
                |> optional "replies" (Decode.map Just (Decode.list commentDecoder)) Nothing
        )
```

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
