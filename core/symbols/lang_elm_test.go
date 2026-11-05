package symbols

import "testing"

func TestExtractElm(t *testing.T) {
	src := `port module Main exposing (main, Model)

import Html exposing (Html, text)

type Msg
    = Increment
    | Decrement Int

type alias Model =
    { count : Int
    , label : String
    }

port sendMessage : String -> Cmd msg

init : Model
init =
    { count = 0, label = "" }

update : Msg -> Model -> Model
update msg model =
    let
        bump x = x + 1
        offset = 2
    in
    case msg of
        Increment ->
            { model | count = bump model.count + offset }

        Decrement n ->
            { model | count = model.count - n }

main : Html msg
main =
    text "ready"
`
	want := []string{
		"Msg enum",
		"Msg.Increment enum_value",
		"Msg.Decrement enum_value",
		"Model type",
		"sendMessage function",
		"init function",
		"update function",
		"main function",
	}
	not := []string{"bump", "offset", "msg", "model", "count", "label", "Html", "text"}
	checkExtract(t, "elm", src, want, not)
}
