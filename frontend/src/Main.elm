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
    , testStatus : Maybe TestStatus
    , currentReport : Maybe Report
    , reportLoading : Bool
    , reportError : Maybe String
    , expandedSections : ExpandedSections
    , screenshotViewer : ScreenshotViewerState
    }


type alias ExpandedSections =
    { issues : Bool
    , recommendations : Bool
    , reasoning : Bool
    , consoleLogs : Bool
    }


type alias ScreenshotViewerState =
    { currentIndex : Int
    , viewMode : ViewMode
    , zoomLevel : ZoomLevel
    , isFullScreen : Bool
    , overlayOpacity : Int
    , loadedImages : List Int
    }


type ViewMode
    = SideBySide
    | Overlay
    | Difference


type ZoomLevel
    = FitToScreen
    | Zoom100
    | Zoom200


type alias TestForm =
    { gameUrl : String
    , maxDuration : Int
    , headless : Bool
    , validationError : Maybe String
    , submitting : Bool
    , submitError : Maybe String
    }


type alias TestStatus =
    { testId : String
    , status : String
    , phase : String
    , progress : Int
    , elapsedTime : Int
    , error : Maybe String
    }


type alias Report =
    { reportId : String
    , gameUrl : String
    , timestamp : String
    , duration : Int
    , score : Maybe PlayabilityScore
    , evidence : Evidence
    , summary : Summary
    , metadata : Maybe (List ( String, String ))
    }


type alias PlayabilityScore =
    { overallScore : Int
    , loadsCorrectly : Bool
    , interactivityScore : Int
    , visualQuality : Int
    , errorSeverity : Int
    , reasoning : String
    , issues : List String
    , recommendations : List String
    }


type alias Evidence =
    { screenshots : List ScreenshotInfo
    , consoleLogs : List ConsoleLog
    , logSummary : LogSummary
    , detectedElements : Maybe (List ( String, String ))
    }


type alias ScreenshotInfo =
    { context : String
    , filepath : String
    , s3Url : Maybe String
    , timestamp : String
    , width : Int
    , height : Int
    }


type alias ConsoleLog =
    { level : String
    , message : String
    , source : String
    , timestamp : String
    }


type alias LogSummary =
    { total : Int
    , errors : Int
    , warnings : Int
    , info : Int
    , debug : Int
    }


type alias Summary =
    { status : String
    , passedChecks : List String
    , failedChecks : List String
    , criticalIssues : List String
    }


type Route
    = Home
    | TestSubmission
    | TestStatusPage String
    | ReportView String
    | TestHistory
    | NotFound


init : () -> Url.Url -> Nav.Key -> ( Model, Cmd Msg )
init flags url key =
    let
        route =
            parseUrl url

        cmd =
            case route of
                ReportView reportId ->
                    fetchReport ("http://localhost:8080/api") reportId

                _ ->
                    Cmd.none
    in
    ( { key = key
      , url = url
      , route = route
      , apiBaseUrl = "http://localhost:8080/api"  -- Update with actual API URL
      , testForm = initTestForm
      , testStatus = Nothing
      , currentReport = Nothing
      , reportLoading = False
      , reportError = Nothing
      , expandedSections = initExpandedSections
      , screenshotViewer = initScreenshotViewer
      }
    , cmd
    )


initExpandedSections : ExpandedSections
initExpandedSections =
    { issues = False
    , recommendations = False
    , reasoning = False
    , consoleLogs = False
    }


initScreenshotViewer : ScreenshotViewerState
initScreenshotViewer =
    { currentIndex = 0
    , viewMode = SideBySide
    , zoomLevel = FitToScreen
    , isFullScreen = False
    , overlayOpacity = 50
    , loadedImages = []
    }


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
        , Parser.map TestStatusPage (Parser.s "test" </> string)
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
    | PollStatus String
    | StatusUpdated (Result Http.Error TestStatus)
    | Tick Time.Posix
    | FetchReport String
    | ReportFetched (Result Http.Error Report)
    | ToggleSection SectionType
    | CopyReportLink String
    | DownloadReportJson Report
    | RerunTest String
    | SelectScreenshot Int
    | ChangeViewMode ViewMode
    | ChangeZoomLevel ZoomLevel
    | ToggleFullScreen
    | AdjustOverlayOpacity Int
    | ImageLoaded Int
    | PreviousScreenshot
    | NextScreenshot


type SectionType
    = IssuesSection
    | RecommendationsSection
    | ReasoningSection
    | ConsoleLogsSection


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
            let
                newRoute =
                    parseUrl url

                cmd =
                    case newRoute of
                        ReportView reportId ->
                            fetchReport model.apiBaseUrl reportId

                        _ ->
                            Cmd.none
            in
            ( { model
                | url = url
                , route = newRoute
                , testForm = initTestForm
                , currentReport = Nothing
                , reportLoading = False
                , reportError = Nothing
              }
            , cmd
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

        PollStatus testId ->
            ( model, pollTestStatus model.apiBaseUrl testId )

        StatusUpdated result ->
            case result of
                Ok status ->
                    ( { model | testStatus = Just status }, Cmd.none )

                Err _ ->
                    ( model, Cmd.none )

        Tick _ ->
            case model.route of
                TestStatusPage testId ->
                    ( model, pollTestStatus model.apiBaseUrl testId )

                _ ->
                    ( model, Cmd.none )

        FetchReport reportId ->
            ( { model | reportLoading = True, reportError = Nothing }
            , fetchReport model.apiBaseUrl reportId
            )

        ReportFetched result ->
            case result of
                Ok report ->
                    ( { model
                        | currentReport = Just report
                        , reportLoading = False
                        , reportError = Nothing
                      }
                    , Cmd.none
                    )

                Err httpError ->
                    let
                        errorMsg =
                            case httpError of
                                Http.BadUrl url ->
                                    "Invalid URL: " ++ url

                                Http.Timeout ->
                                    "Request timed out"

                                Http.NetworkError ->
                                    "Network error occurred"

                                Http.BadStatus status ->
                                    "Server returned error: " ++ String.fromInt status

                                Http.BadBody body ->
                                    "Failed to parse response: " ++ body
                    in
                    ( { model
                        | reportLoading = False
                        , reportError = Just errorMsg
                      }
                    , Cmd.none
                    )

        ToggleSection sectionType ->
            let
                sections =
                    model.expandedSections

                updatedSections =
                    case sectionType of
                        IssuesSection ->
                            { sections | issues = not sections.issues }

                        RecommendationsSection ->
                            { sections | recommendations = not sections.recommendations }

                        ReasoningSection ->
                            { sections | reasoning = not sections.reasoning }

                        ConsoleLogsSection ->
                            { sections | consoleLogs = not sections.consoleLogs }
            in
            ( { model | expandedSections = updatedSections }, Cmd.none )

        CopyReportLink reportId ->
            -- In a real app, this would use the Clipboard API via ports
            ( model, Cmd.none )

        DownloadReportJson report ->
            -- In a real app, this would trigger a download via ports
            ( model, Cmd.none )

        RerunTest gameUrl ->
            let
                form =
                    model.testForm

                updatedForm =
                    { form | gameUrl = gameUrl }
            in
            ( { model | testForm = updatedForm, route = TestSubmission }
            , Nav.pushUrl model.key "/submit"
            )

        SelectScreenshot index ->
            let
                viewer =
                    model.screenshotViewer

                updatedViewer =
                    { viewer | currentIndex = index }
            in
            ( { model | screenshotViewer = updatedViewer }, Cmd.none )

        ChangeViewMode viewMode ->
            let
                viewer =
                    model.screenshotViewer

                updatedViewer =
                    { viewer | viewMode = viewMode }
            in
            ( { model | screenshotViewer = updatedViewer }, Cmd.none )

        ChangeZoomLevel zoomLevel ->
            let
                viewer =
                    model.screenshotViewer

                updatedViewer =
                    { viewer | zoomLevel = zoomLevel }
            in
            ( { model | screenshotViewer = updatedViewer }, Cmd.none )

        ToggleFullScreen ->
            let
                viewer =
                    model.screenshotViewer

                updatedViewer =
                    { viewer | isFullScreen = not viewer.isFullScreen }
            in
            ( { model | screenshotViewer = updatedViewer }, Cmd.none )

        AdjustOverlayOpacity opacity ->
            let
                viewer =
                    model.screenshotViewer

                updatedViewer =
                    { viewer | overlayOpacity = opacity }
            in
            ( { model | screenshotViewer = updatedViewer }, Cmd.none )

        ImageLoaded index ->
            let
                viewer =
                    model.screenshotViewer

                updatedViewer =
                    { viewer | loadedImages = index :: viewer.loadedImages }
            in
            ( { model | screenshotViewer = updatedViewer }, Cmd.none )

        PreviousScreenshot ->
            case model.currentReport of
                Just report ->
                    let
                        viewer =
                            model.screenshotViewer

                        totalScreenshots =
                            List.length report.evidence.screenshots

                        newIndex =
                            if viewer.currentIndex > 0 then
                                viewer.currentIndex - 1

                            else
                                totalScreenshots - 1

                        updatedViewer =
                            { viewer | currentIndex = newIndex }
                    in
                    ( { model | screenshotViewer = updatedViewer }, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )

        NextScreenshot ->
            case model.currentReport of
                Just report ->
                    let
                        viewer =
                            model.screenshotViewer

                        totalScreenshots =
                            List.length report.evidence.screenshots

                        newIndex =
                            remainderBy totalScreenshots (viewer.currentIndex + 1)

                        updatedViewer =
                            { viewer | currentIndex = newIndex }
                    in
                    ( { model | screenshotViewer = updatedViewer }, Cmd.none )

                Nothing ->
                    ( model, Cmd.none )


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


pollTestStatus : String -> String -> Cmd Msg
pollTestStatus apiBaseUrl testId =
    getWithCors
        (apiBaseUrl ++ "/tests/" ++ testId)
        testStatusDecoder
        StatusUpdated


testStatusDecoder : Decode.Decoder TestStatus
testStatusDecoder =
    Decode.map6 TestStatus
        (Decode.field "testId" Decode.string)
        (Decode.field "status" Decode.string)
        (Decode.field "phase" Decode.string)
        (Decode.field "progress" Decode.int)
        (Decode.field "elapsedTime" Decode.int)
        (Decode.maybe (Decode.field "error" Decode.string))


fetchReport : String -> String -> Cmd Msg
fetchReport apiBaseUrl reportId =
    getWithCors
        (apiBaseUrl ++ "/reports/" ++ reportId)
        reportDecoder
        ReportFetched


reportDecoder : Decode.Decoder Report
reportDecoder =
    Decode.map8 Report
        (Decode.field "report_id" Decode.string)
        (Decode.field "game_url" Decode.string)
        (Decode.field "timestamp" Decode.string)
        (Decode.field "duration_ms" Decode.int)
        (Decode.maybe (Decode.field "score" playabilityScoreDecoder))
        (Decode.field "evidence" evidenceDecoder)
        (Decode.field "summary" summaryDecoder)
        (Decode.maybe (Decode.field "metadata" (Decode.keyValuePairs Decode.string) |> Decode.map Just) |> Decode.map (Maybe.withDefault Nothing))


playabilityScoreDecoder : Decode.Decoder PlayabilityScore
playabilityScoreDecoder =
    Decode.map8 PlayabilityScore
        (Decode.field "overall_score" Decode.int)
        (Decode.field "loads_correctly" Decode.bool)
        (Decode.field "interactivity_score" Decode.int)
        (Decode.field "visual_quality" Decode.int)
        (Decode.field "error_severity" Decode.int)
        (Decode.field "reasoning" Decode.string)
        (Decode.field "issues" (Decode.list Decode.string))
        (Decode.field "recommendations" (Decode.list Decode.string))


evidenceDecoder : Decode.Decoder Evidence
evidenceDecoder =
    Decode.map4 Evidence
        (Decode.field "screenshots" (Decode.list screenshotInfoDecoder))
        (Decode.field "console_logs" (Decode.list consoleLogDecoder))
        (Decode.field "log_summary" logSummaryDecoder)
        (Decode.maybe (Decode.field "detected_elements" (Decode.keyValuePairs Decode.string) |> Decode.map Just) |> Decode.map (Maybe.withDefault Nothing))


screenshotInfoDecoder : Decode.Decoder ScreenshotInfo
screenshotInfoDecoder =
    Decode.map6 ScreenshotInfo
        (Decode.field "context" Decode.string)
        (Decode.field "filepath" Decode.string)
        (Decode.maybe (Decode.field "s3_url" Decode.string))
        (Decode.field "timestamp" Decode.string)
        (Decode.field "width" Decode.int)
        (Decode.field "height" Decode.int)


consoleLogDecoder : Decode.Decoder ConsoleLog
consoleLogDecoder =
    Decode.map4 ConsoleLog
        (Decode.field "level" Decode.string)
        (Decode.field "message" Decode.string)
        (Decode.field "source" Decode.string)
        (Decode.field "timestamp" Decode.string)


logSummaryDecoder : Decode.Decoder LogSummary
logSummaryDecoder =
    Decode.map5 LogSummary
        (Decode.field "total" Decode.int)
        (Decode.field "errors" Decode.int)
        (Decode.field "warnings" Decode.int)
        (Decode.field "info" Decode.int)
        (Decode.field "debug" Decode.int)


summaryDecoder : Decode.Decoder Summary
summaryDecoder =
    Decode.map4 Summary
        (Decode.field "status" Decode.string)
        (Decode.field "passed_checks" (Decode.list Decode.string))
        (Decode.field "failed_checks" (Decode.list Decode.string))
        (Decode.field "critical_issues" (Decode.list Decode.string))


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
    case model.route of
        TestStatusPage _ ->
            Time.every 3000 Tick

        _ ->
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

            TestStatusPage testId ->
                viewTestStatus model testId

            ReportView reportId ->
                viewReportView model reportId

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


viewTestStatus : Model -> String -> Html Msg
viewTestStatus model testId =
    div [ class "page test-status" ]
        [ h2 [] [ text "Test Execution Status" ]
        , case model.testStatus of
            Just status ->
                viewStatusDetails status

            Nothing ->
                div [ class "loading" ]
                    [ div [ class "spinner" ] []
                    , p [] [ text "Loading test status..." ]
                    ]
        ]


viewStatusDetails : TestStatus -> Html Msg
viewStatusDetails status =
    div [ class "status-details" ]
        [ div [ class "status-header" ]
            [ div [ class "status-badge", class (statusClass status.status) ]
                [ text (String.toUpper status.status) ]
            , p [ class "test-id" ] [ text ("Test ID: " ++ status.testId) ]
            ]
        , div [ class "status-info" ]
            [ div [ class "info-item" ]
                [ span [ class "label" ] [ text "Phase:" ]
                , span [ class "value" ] [ text status.phase ]
                ]
            , div [ class "info-item" ]
                [ span [ class "label" ] [ text "Elapsed Time:" ]
                , span [ class "value" ] [ text (formatTime status.elapsedTime) ]
                ]
            ]
        , div [ class "progress-section" ]
            [ div [ class "progress-label" ]
                [ text ("Progress: " ++ String.fromInt status.progress ++ "%") ]
            , div [ class "progress-bar-container" ]
                [ div
                    [ class "progress-bar-fill"
                    , style "width" (String.fromInt status.progress ++ "%")
                    ]
                    []
                ]
            ]
        , case status.error of
            Just error ->
                div [ class "error" ] [ text ("Error: " ++ error) ]

            Nothing ->
                text ""
        , if status.status == "completed" then
            div [ class "actions" ]
                [ a [ href ("/report/" ++ status.testId), class "button primary" ]
                    [ text "View Report" ]
                , a [ href "/", class "button secondary" ] [ text "Back to Home" ]
                ]

          else if status.status == "failed" then
            div [ class "actions" ]
                [ a [ href "/", class "button secondary" ] [ text "Back to Home" ]
                , a [ href "/submit", class "button primary" ] [ text "Try Again" ]
                ]

          else
            div [ class "status-message" ]
                [ p [] [ text "Test is running... This page will update automatically." ]
                ]
        ]


statusClass : String -> String
statusClass status =
    case status of
        "completed" ->
            "status-success"

        "failed" ->
            "status-error"

        "running" ->
            "status-running"

        _ ->
            "status-pending"


formatTime : Int -> String
formatTime seconds =
    let
        mins =
            seconds // 60

        secs =
            modBy 60 seconds
    in
    String.fromInt mins ++ "m " ++ String.fromInt secs ++ "s"


viewReportView : Model -> String -> Html Msg
viewReportView model reportId =
    div [ class "page report-view" ]
        [ if model.reportLoading then
            div [ class "loading" ]
                [ div [ class "spinner" ] []
                , p [] [ text "Loading report..." ]
                ]

          else
            case model.reportError of
                Just error ->
                    div [ class "error" ]
                        [ h2 [] [ text "Error Loading Report" ]
                        , p [] [ text error ]
                        , button [ onClick (FetchReport reportId) ] [ text "Retry" ]
                        ]

                Nothing ->
                    case model.currentReport of
                        Just report ->
                            viewReport model report

                        Nothing ->
                            div []
                                [ h2 [] [ text ("Report: " ++ reportId) ]
                                , p [] [ text "No report data available." ]
                                ]
        ]


viewReport : Model -> Report -> Html Msg
viewReport model report =
    div [ class "report-container" ]
        [ -- Header and Summary (Subtask 1)
          viewReportHeader report
        , viewReportSummary report

        -- Metrics Visualization (Subtask 2)
        , case report.score of
            Just score ->
                viewMetrics score

            Nothing ->
                div [ class "no-metrics" ]
                    [ p [] [ text "No evaluation metrics available." ]
                    ]

        -- Screenshot Viewer (Task 5)
        , if List.isEmpty report.evidence.screenshots |> not then
            viewScreenshotViewer model report.evidence.screenshots

          else
            text ""

        -- Collapsible Issues and Recommendations (Subtask 3)
        , case report.score of
            Just score ->
                div []
                    [ viewCollapsibleSection model.expandedSections.issues IssuesSection "Issues Found" (viewIssues score.issues)
                    , viewCollapsibleSection model.expandedSections.recommendations RecommendationsSection "Recommendations" (viewRecommendations score.recommendations)
                    , viewCollapsibleSection model.expandedSections.reasoning ReasoningSection "AI Reasoning" (viewReasoning score.reasoning)
                    ]

            Nothing ->
                text ""

        -- Console Logs (part of subtask 3)
        , viewCollapsibleSection model.expandedSections.consoleLogs ConsoleLogsSection ("Console Logs (" ++ String.fromInt report.evidence.logSummary.total ++ ")") (viewConsoleLogs report.evidence)

        -- Report Actions (Subtask 4)
        , viewReportActions report
        ]


{-| Subtask 1: Report Header and Summary -}
viewReportHeader : Report -> Html Msg
viewReportHeader report =
    div [ class "report-header" ]
        [ div [ class "header-main" ]
            [ h2 [] [ text "Test Report" ]
            , div [ class ("status-badge status-" ++ String.toLower report.summary.status) ]
                [ text (String.toUpper report.summary.status) ]
            ]
        , div [ class "header-info" ]
            [ div [ class "info-item" ]
                [ span [ class "label" ] [ text "Game URL:" ]
                , a [ href report.gameUrl, class "game-link" ] [ text report.gameUrl ]
                ]
            , div [ class "info-item" ]
                [ span [ class "label" ] [ text "Report ID:" ]
                , span [ class "value" ] [ text report.reportId ]
                ]
            , div [ class "info-item" ]
                [ span [ class "label" ] [ text "Timestamp:" ]
                , span [ class "value" ] [ text report.timestamp ]
                ]
            , div [ class "info-item" ]
                [ span [ class "label" ] [ text "Duration:" ]
                , span [ class "value" ] [ text (formatDuration report.duration) ]
                ]
            , case report.score of
                Just score ->
                    div [ class "info-item" ]
                        [ span [ class "label" ] [ text "Overall Score:" ]
                        , span [ class ("score-value " ++ scoreClass score.overallScore) ]
                            [ text (String.fromInt score.overallScore ++ "/100") ]
                        ]

                Nothing ->
                    text ""
            ]
        ]


viewReportSummary : Report -> Html Msg
viewReportSummary report =
    div [ class "report-summary" ]
        [ h3 [] [ text "Summary" ]
        , div [ class "summary-grid" ]
            [ if List.isEmpty report.summary.passedChecks |> not then
                div [ class "summary-section passed" ]
                    [ h4 [] [ text ("✓ Passed (" ++ String.fromInt (List.length report.summary.passedChecks) ++ ")") ]
                    , ul [] (List.map (\check -> li [] [ text check ]) report.summary.passedChecks)
                    ]

              else
                text ""
            , if List.isEmpty report.summary.failedChecks |> not then
                div [ class "summary-section failed" ]
                    [ h4 [] [ text ("✗ Failed (" ++ String.fromInt (List.length report.summary.failedChecks) ++ ")") ]
                    , ul [] (List.map (\check -> li [] [ text check ]) report.summary.failedChecks)
                    ]

              else
                text ""
            , if List.isEmpty report.summary.criticalIssues |> not then
                div [ class "summary-section critical" ]
                    [ h4 [] [ text ("⚠ Critical Issues (" ++ String.fromInt (List.length report.summary.criticalIssues) ++ ")") ]
                    , ul [] (List.map (\issue -> li [] [ text issue ]) report.summary.criticalIssues)
                    ]

              else
                text ""
            ]
        ]


{-| Subtask 2: Metrics Visualization with Progress Bars -}
viewMetrics : PlayabilityScore -> Html Msg
viewMetrics score =
    div [ class "metrics-section" ]
        [ h3 [] [ text "Evaluation Metrics" ]
        , div [ class "metrics-grid" ]
            [ viewMetricBar "Overall Score" score.overallScore
            , viewMetricBar "Interactivity" score.interactivityScore
            , viewMetricBar "Visual Quality" score.visualQuality
            , viewMetricBar "Error Severity" (100 - score.errorSeverity) -- Invert so higher is better
            , div [ class "metric-item boolean" ]
                [ span [ class "metric-label" ] [ text "Loads Correctly" ]
                , span [ class ("metric-value " ++ if score.loadsCorrectly then "success" else "error") ]
                    [ text (if score.loadsCorrectly then "✓ Yes" else "✗ No") ]
                ]
            ]
        ]


viewMetricBar : String -> Int -> Html Msg
viewMetricBar label value =
    div [ class "metric-item" ]
        [ div [ class "metric-label-row" ]
            [ span [ class "metric-label" ] [ text label ]
            , span [ class ("metric-score " ++ scoreClass value) ]
                [ text (String.fromInt value ++ "/100") ]
            ]
        , div [ class "progress-bar-container" ]
            [ div
                [ class ("progress-bar " ++ scoreClass value)
                , style "width" (String.fromInt value ++ "%")
                ]
                []
            ]
        ]


{-| Subtask 3: Collapsible Issues and Recommendations -}
viewCollapsibleSection : Bool -> SectionType -> String -> Html Msg -> Html Msg
viewCollapsibleSection isExpanded sectionType title content =
    div [ class "collapsible-section" ]
        [ div
            [ class "section-header"
            , onClick (ToggleSection sectionType)
            ]
            [ h3 [] [ text title ]
            , span [ class "toggle-icon" ]
                [ text (if isExpanded then "▼" else "▶") ]
            ]
        , if isExpanded then
            div [ class "section-content" ] [ content ]

          else
            text ""
        ]


viewIssues : List String -> Html Msg
viewIssues issues =
    if List.isEmpty issues then
        div [ class "empty-state" ] [ text "No issues found!" ]

    else
        ul [ class "issues-list" ]
            (List.map (\issue -> li [ class "issue-item" ] [ text ("• " ++ issue) ]) issues)


viewRecommendations : List String -> Html Msg
viewRecommendations recommendations =
    if List.isEmpty recommendations then
        div [ class "empty-state" ] [ text "No recommendations at this time." ]

    else
        ul [ class "recommendations-list" ]
            (List.map (\rec -> li [ class "recommendation-item" ] [ text ("→ " ++ rec) ]) recommendations)


viewReasoning : String -> Html Msg
viewReasoning reasoning =
    div [ class "reasoning-text" ]
        [ p [] [ text reasoning ] ]


viewConsoleLogs : Evidence -> Html Msg
viewConsoleLogs evidence =
    div []
        [ div [ class "log-summary" ]
            [ span [ class "log-stat error" ] [ text ("Errors: " ++ String.fromInt evidence.logSummary.errors) ]
            , span [ class "log-stat warning" ] [ text ("Warnings: " ++ String.fromInt evidence.logSummary.warnings) ]
            , span [ class "log-stat info" ] [ text ("Info: " ++ String.fromInt evidence.logSummary.info) ]
            ]
        , if List.isEmpty evidence.consoleLogs then
            div [ class "empty-state" ] [ text "No console logs captured." ]

          else
            div [ class "console-logs" ]
                (List.take 50 evidence.consoleLogs
                    |> List.map viewConsoleLog
                )
        ]


viewConsoleLog : ConsoleLog -> Html Msg
viewConsoleLog log =
    div [ class ("console-log log-" ++ log.level) ]
        [ span [ class "log-level" ] [ text (String.toUpper log.level) ]
        , span [ class "log-message" ] [ text log.message ]
        , span [ class "log-source" ] [ text log.source ]
        ]


{-| Subtask 4: Report Actions -}
viewReportActions : Report -> Html Msg
viewReportActions report =
    div [ class "report-actions" ]
        [ h3 [] [ text "Actions" ]
        , div [ class "action-buttons" ]
            [ button
                [ class "action-btn primary"
                , onClick (CopyReportLink report.reportId)
                ]
                [ text "📋 Copy Share Link" ]
            , button
                [ class "action-btn secondary"
                , onClick (DownloadReportJson report)
                ]
                [ text "💾 Download JSON" ]
            , button
                [ class "action-btn secondary"
                , onClick (RerunTest report.gameUrl)
                ]
                [ text "🔄 Re-run Test" ]
            ]
        ]


{-| Task 5: Screenshot Viewer with Lazy Loading, View Modes, Zoom, and Metadata -}
viewScreenshotViewer : Model -> List ScreenshotInfo -> Html Msg
viewScreenshotViewer model screenshots =
    let
        viewer =
            model.screenshotViewer

        currentScreenshot =
            List.drop viewer.currentIndex screenshots
                |> List.head
    in
    div
        [ class
            (if viewer.isFullScreen then
                "screenshot-viewer fullscreen"

             else
                "screenshot-viewer"
            )
        ]
        [ h3 [] [ text ("Screenshots (" ++ String.fromInt (List.length screenshots) ++ ")") ]
        , case currentScreenshot of
            Just screenshot ->
                div [ class "viewer-container" ]
                    [ -- Toolbar (Subtask 3: Zoom Controls + View Modes)
                      viewScreenshotToolbar viewer

                    -- Main Viewer Area (Subtask 1 & 2: Lazy Loading + View Modes)
                    , viewScreenshotDisplay viewer screenshot screenshots

                    -- Thumbnail Navigation (Subtask 1: Lazy Loading)
                    , viewThumbnailStrip model screenshots

                    -- Metadata Display (Subtask 4)
                    , viewScreenshotMetadata screenshot
                    ]

            Nothing ->
                div [ class "no-screenshots" ]
                    [ text "No screenshots available" ]
        ]


{-| Subtask 3: Screenshot Toolbar with Zoom and View Mode Controls -}
viewScreenshotToolbar : ScreenshotViewerState -> Html Msg
viewScreenshotToolbar viewer =
    div [ class "screenshot-toolbar" ]
        [ div [ class "toolbar-group view-modes" ]
            [ span [ class "toolbar-label" ] [ text "View:" ]
            , button
                [ class
                    (if viewer.viewMode == SideBySide then
                        "btn-mode active"

                     else
                        "btn-mode"
                    )
                , onClick (ChangeViewMode SideBySide)
                ]
                [ text "Side-by-Side" ]
            , button
                [ class
                    (if viewer.viewMode == Overlay then
                        "btn-mode active"

                     else
                        "btn-mode"
                    )
                , onClick (ChangeViewMode Overlay)
                ]
                [ text "Overlay" ]
            , button
                [ class
                    (if viewer.viewMode == Difference then
                        "btn-mode active"

                     else
                        "btn-mode"
                    )
                , onClick (ChangeViewMode Difference)
                ]
                [ text "Difference" ]
            ]
        , div [ class "toolbar-group zoom-controls" ]
            [ span [ class "toolbar-label" ] [ text "Zoom:" ]
            , button
                [ class
                    (if viewer.zoomLevel == FitToScreen then
                        "btn-zoom active"

                     else
                        "btn-zoom"
                    )
                , onClick (ChangeZoomLevel FitToScreen)
                ]
                [ text "Fit" ]
            , button
                [ class
                    (if viewer.zoomLevel == Zoom100 then
                        "btn-zoom active"

                     else
                        "btn-zoom"
                    )
                , onClick (ChangeZoomLevel Zoom100)
                ]
                [ text "100%" ]
            , button
                [ class
                    (if viewer.zoomLevel == Zoom200 then
                        "btn-zoom active"

                     else
                        "btn-zoom"
                    )
                , onClick (ChangeZoomLevel Zoom200)
                ]
                [ text "200%" ]
            ]
        , div [ class "toolbar-group fullscreen-control" ]
            [ button [ class "btn-fullscreen", onClick ToggleFullScreen ]
                [ text
                    (if viewer.isFullScreen then
                        "⛶ Exit Fullscreen"

                     else
                        "⛶ Fullscreen"
                    )
                ]
            ]
        ]


{-| Subtask 2: Screenshot Display with View Modes -}
viewScreenshotDisplay : ScreenshotViewerState -> ScreenshotInfo -> List ScreenshotInfo -> Html Msg
viewScreenshotDisplay viewer current allScreenshots =
    let
        zoomClass =
            case viewer.zoomLevel of
                FitToScreen ->
                    "zoom-fit"

                Zoom100 ->
                    "zoom-100"

                Zoom200 ->
                    "zoom-200"

        imageUrl =
            Maybe.withDefault current.filepath current.s3Url
    in
    div [ class ("screenshot-display " ++ zoomClass) ]
        [ case viewer.viewMode of
            SideBySide ->
                viewSideBySide current allScreenshots

            Overlay ->
                viewOverlay viewer current allScreenshots

            Difference ->
                viewDifference current allScreenshots
        , div [ class "navigation-controls" ]
            [ button [ class "btn-nav prev", onClick PreviousScreenshot ] [ text "◀ Previous" ]
            , span [ class "screenshot-counter" ]
                [ text (String.fromInt (viewer.currentIndex + 1) ++ " / " ++ String.fromInt (List.length allScreenshots)) ]
            , button [ class "btn-nav next", onClick NextScreenshot ] [ text "Next ▶" ]
            ]
        ]


viewSideBySide : ScreenshotInfo -> List ScreenshotInfo -> Html Msg
viewSideBySide current allScreenshots =
    let
        previous =
            List.head allScreenshots

        imageUrl =
            Maybe.withDefault current.filepath current.s3Url

        prevUrl =
            previous
                |> Maybe.andThen (\p -> Maybe.withDefault p.filepath p.s3Url |> Just)
                |> Maybe.withDefault imageUrl
    in
    div [ class "view-sidebyside" ]
        [ div [ class "side-image" ]
            [ div [ class "image-label" ] [ text "Before" ]
            , img [ src prevUrl, alt "Before screenshot" ] []
            ]
        , div [ class "side-image" ]
            [ div [ class "image-label" ] [ text "Current" ]
            , img [ src imageUrl, alt "Current screenshot" ] []
            ]
        ]


viewOverlay : ScreenshotViewerState -> ScreenshotInfo -> List ScreenshotInfo -> Html Msg
viewOverlay viewer current allScreenshots =
    let
        previous =
            List.head allScreenshots

        imageUrl =
            Maybe.withDefault current.filepath current.s3Url

        prevUrl =
            previous
                |> Maybe.andThen (\p -> Maybe.withDefault p.filepath p.s3Url |> Just)
                |> Maybe.withDefault imageUrl

        opacityValue =
            toFloat viewer.overlayOpacity / 100
    in
    div [ class "view-overlay" ]
        [ div [ class "overlay-container" ]
            [ img [ src prevUrl, alt "Base screenshot", class "overlay-base" ] []
            , img
                [ src imageUrl
                , alt "Overlay screenshot"
                , class "overlay-top"
                , style "opacity" (String.fromFloat opacityValue)
                ]
                []
            ]
        , div [ class "overlay-slider" ]
            [ span [] [ text "Opacity:" ]
            , input
                [ type_ "range"
                , Html.Attributes.min "0"
                , Html.Attributes.max "100"
                , value (String.fromInt viewer.overlayOpacity)
                , onInput (String.toInt >> Maybe.withDefault 50 >> AdjustOverlayOpacity)
                ]
                []
            , span [] [ text (String.fromInt viewer.overlayOpacity ++ "%") ]
            ]
        ]


viewDifference : ScreenshotInfo -> List ScreenshotInfo -> Html Msg
viewDifference current allScreenshots =
    let
        imageUrl =
            Maybe.withDefault current.filepath current.s3Url
    in
    div [ class "view-difference" ]
        [ div [ class "difference-notice" ]
            [ text "⚠ Difference mode requires canvas processing (not yet implemented)" ]
        , img [ src imageUrl, alt "Screenshot", class "difference-image" ] []
        ]


{-| Subtask 1: Thumbnail Strip with Lazy Loading -}
viewThumbnailStrip : Model -> List ScreenshotInfo -> Html Msg
viewThumbnailStrip model screenshots =
    div [ class "thumbnail-strip" ]
        (List.indexedMap (viewThumbnail model) screenshots)


viewThumbnail : Model -> Int -> ScreenshotInfo -> Html Msg
viewThumbnail model index screenshot =
    let
        viewer =
            model.screenshotViewer

        isActive =
            viewer.currentIndex == index

        isLoaded =
            List.member index viewer.loadedImages

        imageUrl =
            Maybe.withDefault screenshot.filepath screenshot.s3Url
    in
    div
        [ class
            (if isActive then
                "thumbnail active"

             else
                "thumbnail"
            )
        , onClick (SelectScreenshot index)
        ]
        [ if isLoaded then
            img
                [ src imageUrl
                , alt ("Screenshot " ++ String.fromInt (index + 1))
                , class "thumbnail-image"
                ]
                []

          else
            div [ class "thumbnail-placeholder" ]
                [ text "..." ]
        , div [ class "thumbnail-label" ] [ text (String.fromInt (index + 1)) ]
        ]


{-| Subtask 4: Screenshot Metadata Display -}
viewScreenshotMetadata : ScreenshotInfo -> Html Msg
viewScreenshotMetadata screenshot =
    div [ class "screenshot-metadata" ]
        [ h4 [] [ text "Metadata" ]
        , div [ class "metadata-grid" ]
            [ div [ class "metadata-item" ]
                [ span [ class "metadata-label" ] [ text "Context:" ]
                , span [ class "metadata-value" ] [ text screenshot.context ]
                ]
            , div [ class "metadata-item" ]
                [ span [ class "metadata-label" ] [ text "Timestamp:" ]
                , span [ class "metadata-value" ] [ text screenshot.timestamp ]
                ]
            , div [ class "metadata-item" ]
                [ span [ class "metadata-label" ] [ text "Resolution:" ]
                , span [ class "metadata-value" ]
                    [ text (String.fromInt screenshot.width ++ " × " ++ String.fromInt screenshot.height) ]
                ]
            , div [ class "metadata-item" ]
                [ span [ class "metadata-label" ] [ text "File:" ]
                , span [ class "metadata-value" ] [ text screenshot.filepath ]
                ]
            , case screenshot.s3Url of
                Just url ->
                    div [ class "metadata-item" ]
                        [ span [ class "metadata-label" ] [ text "S3 URL:" ]
                        , a [ href url, class "metadata-link", Html.Attributes.target "_blank" ] [ text "View on S3" ]
                        ]

                Nothing ->
                    text ""
            ]
        ]


{-| Helper Functions -}
scoreClass : Int -> String
scoreClass score =
    if score >= 80 then
        "score-good"

    else if score >= 50 then
        "score-medium"

    else
        "score-poor"


formatDuration : Int -> String
formatDuration durationMs =
    let
        seconds =
            durationMs // 1000

        mins =
            seconds // 60

        secs =
            remainderBy 60 seconds
    in
    if mins > 0 then
        String.fromInt mins ++ "m " ++ String.fromInt secs ++ "s"

    else
        String.fromInt secs ++ "s"


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
