module Main exposing (main)

import Browser
import Browser.Navigation as Nav
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (onClick, onInput, onSubmit)
import Http
import Json.Decode as Decode
import Json.Encode as Encode
import Time
import Url
import Url.Parser as Parser exposing ((</>), Parser, string)


-- MAIN


main : Program () Model Msg
main =
    Browser.application
        { init = init
        , view = view
        , update = update
        , subscriptions = subscriptions
        , onUrlChange = UrlChanged
        , onUrlRequest = LinkClicked
        }


-- MODEL


type alias Model =
    { key : Nav.Key
    , url : Url.Url
    , route : Route
    , apiBaseUrl : String
    , testForm : TestForm
    }


type alias TestForm =
    { gameUrl : String
    , maxDuration : Int
    , headless : Bool
    , validationError : Maybe String
    , submitting : Bool
    , submitError : Maybe String
    }


type Route
    = Home
    | TestSubmission
    | TestStatus String
    | ReportView String
    | TestHistory
    | NotFound


init : () -> Url.Url -> Nav.Key -> ( Model, Cmd Msg )
init flags url key =
    ( { key = key
      , url = url
      , route = parseUrl url
      , apiBaseUrl = "http://localhost:8080/api"  -- Update with actual API URL
      , testForm = initTestForm
      }
    , Cmd.none
    )


initTestForm : TestForm
initTestForm =
    { gameUrl = ""
    , maxDuration = 60
    , headless = False
    , validationError = Nothing
    , submitting = False
    , submitError = Nothing
    }


-- URL PARSING


routeParser : Parser (Route -> a) a
routeParser =
    Parser.oneOf
        [ Parser.map Home Parser.top
        , Parser.map TestSubmission (Parser.s "submit")
        , Parser.map TestStatus (Parser.s "test" </> string)
        , Parser.map ReportView (Parser.s "report" </> string)
        , Parser.map TestHistory (Parser.s "history")
        ]


parseUrl : Url.Url -> Route
parseUrl url =
    Parser.parse routeParser url
        |> Maybe.withDefault NotFound


-- UPDATE


type Msg
    = NoOp
    | LinkClicked Browser.UrlRequest
    | UrlChanged Url.Url
    | UpdateGameUrl String
    | UpdateMaxDuration String
    | ToggleHeadless
    | SubmitTest
    | TestSubmitted (Result Http.Error TestSubmitResponse)


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        NoOp ->
            ( model, Cmd.none )

        LinkClicked urlRequest ->
            case urlRequest of
                Browser.Internal url ->
                    ( model, Nav.pushUrl model.key (Url.toString url) )

                Browser.External href ->
                    ( model, Nav.load href )

        UrlChanged url ->
            ( { model | url = url, route = parseUrl url, testForm = initTestForm }
            , Cmd.none
            )

        UpdateGameUrl newUrl ->
            let
                form =
                    model.testForm

                updatedForm =
                    { form | gameUrl = newUrl, validationError = Nothing }
            in
            ( { model | testForm = updatedForm }, Cmd.none )

        UpdateMaxDuration durationStr ->
            let
                form =
                    model.testForm

                duration =
                    String.toInt durationStr |> Maybe.withDefault 60

                updatedForm =
                    { form | maxDuration = duration }
            in
            ( { model | testForm = updatedForm }, Cmd.none )

        ToggleHeadless ->
            let
                form =
                    model.testForm

                updatedForm =
                    { form | headless = not form.headless }
            in
            ( { model | testForm = updatedForm }, Cmd.none )

        SubmitTest ->
            let
                form =
                    model.testForm
            in
            case validateUrl form.gameUrl of
                Just error ->
                    ( { model | testForm = { form | validationError = Just error } }, Cmd.none )

                Nothing ->
                    ( { model | testForm = { form | submitting = True, submitError = Nothing } }
                    , submitTestRequest model.apiBaseUrl form
                    )

        TestSubmitted result ->
            let
                form =
                    model.testForm
            in
            case result of
                Ok response ->
                    ( { model | testForm = initTestForm }
                    , Nav.pushUrl model.key ("/test/" ++ response.testId)
                    )

                Err error ->
                    ( { model
                        | testForm =
                            { form
                                | submitting = False
                                , submitError = Just (httpErrorToString error)
                            }
                      }
                    , Cmd.none
                    )


-- VALIDATION


validateUrl : String -> Maybe String
validateUrl url =
    if String.isEmpty url then
        Just "Please enter a game URL"

    else if not (String.startsWith "http://" url || String.startsWith "https://" url) then
        Just "URL must start with http:// or https://"

    else if String.length url < 10 then
        Just "Please enter a valid URL"

    else
        Nothing


-- API TYPES AND REQUESTS


type alias TestSubmitResponse =
    { testId : String
    , estimatedCompletionTime : Int
    }


submitTestRequest : String -> TestForm -> Cmd Msg
submitTestRequest apiBaseUrl form =
    let
        body =
            Encode.object
                [ ( "url", Encode.string form.gameUrl )
                , ( "maxDuration", Encode.int form.maxDuration )
                , ( "headless", Encode.bool form.headless )
                ]
    in
    postWithCors
        (apiBaseUrl ++ "/tests")
        body
        testSubmitResponseDecoder
        TestSubmitted


testSubmitResponseDecoder : Decode.Decoder TestSubmitResponse
testSubmitResponseDecoder =
    Decode.map2 TestSubmitResponse
        (Decode.field "testId" Decode.string)
        (Decode.field "estimatedCompletionTime" Decode.int)


httpErrorToString : Http.Error -> String
httpErrorToString error =
    case error of
        Http.BadUrl url ->
            "Invalid URL: " ++ url

        Http.Timeout ->
            "Request timed out. Please try again."

        Http.NetworkError ->
            "Network error. Please check your connection."

        Http.BadStatus status ->
            "Server error (status " ++ String.fromInt status ++ ")"

        Http.BadBody body ->
            "Invalid response from server: " ++ body


-- HTTP HELPERS (with CORS support)


{-| Create HTTP request with CORS headers
-}
corsHeaders : List Http.Header
corsHeaders =
    [ Http.header "Access-Control-Allow-Origin" "*"
    , Http.header "Access-Control-Allow-Methods" "GET, POST, PUT, DELETE, OPTIONS"
    , Http.header "Access-Control-Allow-Headers" "Content-Type"
    ]


{-| Make a GET request with CORS headers
-}
getWithCors : String -> Decode.Decoder a -> (Result Http.Error a -> Msg) -> Cmd Msg
getWithCors url decoder toMsg =
    Http.request
        { method = "GET"
        , headers = corsHeaders
        , url = url
        , body = Http.emptyBody
        , expect = Http.expectJson toMsg decoder
        , timeout = Nothing
        , tracker = Nothing
        }


{-| Make a POST request with CORS headers
-}
postWithCors : String -> Encode.Value -> Decode.Decoder a -> (Result Http.Error a -> Msg) -> Cmd Msg
postWithCors url body decoder toMsg =
    Http.request
        { method = "POST"
        , headers = corsHeaders
        , url = url
        , body = Http.jsonBody body
        , expect = Http.expectJson toMsg decoder
        , timeout = Nothing
        , tracker = Nothing
        }


-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none


-- VIEW


view : Model -> Browser.Document Msg
view model =
    { title = "DreamUp QA Agent Dashboard"
    , body =
        [ div [ class "app-container" ]
            [ viewHeader model
            , viewContent model
            , viewFooter
            ]
        ]
    }


viewHeader : Model -> Html Msg
viewHeader model =
    header [ class "header" ]
        [ h1 [] [ text "DreamUp QA Agent" ]
        , nav [ class "navigation" ]
            [ a [ href "/" ] [ text "Home" ]
            , a [ href "/submit" ] [ text "Submit Test" ]
            , a [ href "/history" ] [ text "Test History" ]
            ]
        ]


viewContent : Model -> Html Msg
viewContent model =
    main_ [ class "main-content" ]
        [ case model.route of
            Home ->
                viewHome

            TestSubmission ->
                viewTestSubmission model

            TestStatus testId ->
                viewTestStatus testId

            ReportView reportId ->
                viewReportView reportId

            TestHistory ->
                viewTestHistory

            NotFound ->
                viewNotFound
        ]


viewHome : Html Msg
viewHome =
    div [ class "page home" ]
        [ h2 [] [ text "Welcome to DreamUp QA Agent" ]
        , p [] [ text "An automated QA testing system for web games." ]
        , div [ class "quick-actions" ]
            [ a [ href "/submit", class "button primary" ] [ text "Submit New Test" ]
            , a [ href "/history", class "button secondary" ] [ text "View Test History" ]
            ]
        ]


viewTestSubmission : Model -> Html Msg
viewTestSubmission model =
    let
        form =
            model.testForm
    in
    div [ class "page test-submission" ]
        [ h2 [] [ text "Submit New Test" ]
        , p [] [ text "Enter a game URL to start automated QA testing." ]
        , Html.form [ onSubmit SubmitTest, class "test-form" ]
            [ div [ class "form-group" ]
                [ label [ for "game-url" ] [ text "Game URL" ]
                , input
                    [ type_ "text"
                    , id "game-url"
                    , placeholder "https://example.com/game"
                    , value form.gameUrl
                    , onInput UpdateGameUrl
                    , disabled form.submitting
                    , class "form-input"
                    ]
                    []
                , case form.validationError of
                    Just error ->
                        div [ class "error" ] [ text error ]

                    Nothing ->
                        text ""
                ]
            , div [ class "form-group" ]
                [ label [ for "max-duration" ]
                    [ text ("Max Duration: " ++ String.fromInt form.maxDuration ++ "s") ]
                , input
                    [ type_ "range"
                    , id "max-duration"
                    , Html.Attributes.min "60"
                    , Html.Attributes.max "300"
                    , value (String.fromInt form.maxDuration)
                    , onInput UpdateMaxDuration
                    , disabled form.submitting
                    , class "form-slider"
                    ]
                    []
                , p [ class "help-text" ] [ text "Maximum time allowed for the test (60-300 seconds)" ]
                ]
            , div [ class "form-group" ]
                [ label [ class "checkbox-label" ]
                    [ input
                        [ type_ "checkbox"
                        , checked form.headless
                        , onClick ToggleHeadless
                        , disabled form.submitting
                        ]
                        []
                    , text " Run in headless mode (no visible browser)"
                    ]
                ]
            , case form.submitError of
                Just error ->
                    div [ class "error" ] [ text error ]

                Nothing ->
                    text ""
            , div [ class "form-actions" ]
                [ button
                    [ type_ "submit"
                    , class "button primary"
                    , disabled (form.submitting || String.isEmpty form.gameUrl)
                    ]
                    [ text
                        (if form.submitting then
                            "Submitting..."

                         else
                            "Start Test"
                        )
                    ]
                , a [ href "/", class "button secondary" ] [ text "Cancel" ]
                ]
            ]
        ]


viewTestStatus : String -> Html Msg
viewTestStatus testId =
    div [ class "page test-status" ]
        [ h2 [] [ text ("Test Status: " ++ testId) ]
        , p [] [ text "Test status tracking will be implemented here." ]
        ]


viewReportView : String -> Html Msg
viewReportView reportId =
    div [ class "page report-view" ]
        [ h2 [] [ text ("Report: " ++ reportId) ]
        , p [] [ text "Report display will be implemented here." ]
        ]


viewTestHistory : Html Msg
viewTestHistory =
    div [ class "page test-history" ]
        [ h2 [] [ text "Test History" ]
        , p [] [ text "Test history list will be implemented here." ]
        ]


viewNotFound : Html Msg
viewNotFound =
    div [ class "page not-found" ]
        [ h2 [] [ text "404 - Page Not Found" ]
        , p [] [ text "The page you are looking for does not exist." ]
        , a [ href "/" ] [ text "Go Home" ]
        ]


viewFooter : Html Msg
viewFooter =
    footer [ class "footer" ]
        [ p [] [ text "© 2025 DreamUp QA Agent" ]
        ]
