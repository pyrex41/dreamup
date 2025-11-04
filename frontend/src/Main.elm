module Main exposing (main)

import Browser
import Browser.Navigation as Nav
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (onClick, onInput, onSubmit)
import Http
import Json.Decode as Decode
import Json.Encode as Encode
import Random
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
    , consoleLogViewer : ConsoleLogViewerState
    , testHistory : TestHistoryState
    , networkStatus : NetworkStatus
    , retryState : RetryState
    , batchTestForm : BatchTestForm
    , batchTestStatus : Maybe BatchTestStatus
    }


type alias NetworkStatus =
    { isOnline : Bool
    , lastError : Maybe String
    , lastSuccessfulRequest : Maybe String
    }


type alias RetryState =
    { pendingRetry : Maybe PendingRetry
    , retryCount : Int
    , maxRetries : Int
    }


type alias PendingRetry =
    { requestType : String
    , retryAfter : Int  -- milliseconds
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


type alias ConsoleLogViewerState =
    { searchQuery : String
    , levelFilters : LevelFilters
    , expandedLogs : List Int
    , virtualScrollOffset : Int
    , virtualScrollLimit : Int
    }


type alias LevelFilters =
    { showError : Bool
    , showWarning : Bool
    , showInfo : Bool
    , showDebug : Bool
    , showLog : Bool
    }


type alias TestHistoryState =
    { reports : List ReportSummary
    , loading : Bool
    , error : Maybe String
    , currentPage : Int
    , totalPages : Int
    , itemsPerPage : Int
    , sortBy : SortField
    , sortOrder : SortOrder
    , statusFilter : Maybe String
    , urlSearchQuery : String
    }


type alias ReportSummary =
    { reportId : String
    , gameUrl : String
    , timestamp : String
    , status : String
    , overallScore : Maybe Int
    , duration : Int
    }


type SortField
    = SortByTimestamp
    | SortByScore
    | SortByDuration
    | SortByStatus


type SortOrder
    = Ascending
    | Descending


type alias TestForm =
    { gameUrl : String
    , maxDuration : Int
    , headless : Bool
    , validationError : Maybe String
    , submitting : Bool
    , submitError : Maybe String
    }


type alias BatchTestForm =
    { urls : List String
    , urlInput : String
    , maxDuration : Int
    , headless : Bool
    , validationError : Maybe String
    , submitting : Bool
    , submitError : Maybe String
    }


type alias BatchTestStatus =
    { batchId : String
    , status : String
    , tests : List TestStatus
    , totalTests : Int
    , completedTests : Int
    , failedTests : Int
    , runningTests : Int
    , createdAt : String
    , updatedAt : String
    }


type alias TestStatus =
    { testId : String
    , status : String
    , progress : Int
    , message : String
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
    , videoUrl : Maybe String
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
    | BatchTestSubmission
    | TestStatusPage String
    | BatchTestStatusPage String
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

                TestHistory ->
                    fetchTestHistory ("http://localhost:8080/api") initTestHistory

                _ ->
                    Cmd.none
    in
    ( { key = key
      , url = url
      , route = route
      , apiBaseUrl = "/api"  -- Uses Vite proxy in dev, relative path in production
      , testForm = initTestForm
      , testStatus = Nothing
      , currentReport = Nothing
      , reportLoading = False
      , reportError = Nothing
      , expandedSections = initExpandedSections
      , screenshotViewer = initScreenshotViewer
      , consoleLogViewer = initConsoleLogViewer
      , testHistory = initTestHistory
      , networkStatus = initNetworkStatus
      , retryState = initRetryState
      , batchTestForm = initBatchTestForm
      , batchTestStatus = Nothing
      }
    , cmd
    )


-- GAME LIBRARY


type alias ExampleGame =
    { title : String
    , description : String
    , url : String
    , score : String
    , badgeType : String
    }


gameLibrary : List ExampleGame
gameLibrary =
    [ { title = "Pac-Man"
      , description = "Classic arcade game - eat dots and avoid ghosts"
      , url = "https://freepacman.org"
      , score = "Testing"
      , badgeType = "info"
      }
    , { title = "2048"
      , description = "Number puzzle game - combine tiles to reach 2048"
      , url = "https://play2048.co"
      , score = "Testing"
      , badgeType = "info"
      }
    , { title = "Free Rider 2"
      , description = "Kongregate - Physics-based bike game"
      , url = "https://www.kongregate.com/en/games/onemorelevel/free-rider-2"
      , score = "65/100"
      , badgeType = "success"
      }
    , { title = "Subway Surfers"
      , description = "Poki - Endless runner game"
      , url = "https://www.poki.com/en/g/subway-surfers"
      , score = "40/100"
      , badgeType = "warning"
      }
    , { title = "Agar.io"
      , description = "Simple multiplayer game"
      , url = "https://agar.io"
      , score = "Testing"
      , badgeType = "info"
      }
    ]


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


initConsoleLogViewer : ConsoleLogViewerState
initConsoleLogViewer =
    { searchQuery = ""
    , levelFilters = initLevelFilters
    , expandedLogs = []
    , virtualScrollOffset = 0
    , virtualScrollLimit = 100
    }


initLevelFilters : LevelFilters
initLevelFilters =
    { showError = True
    , showWarning = True
    , showInfo = True
    , showDebug = True
    , showLog = True
    }


initTestHistory : TestHistoryState
initTestHistory =
    { reports = []
    , loading = False
    , error = Nothing
    , currentPage = 1
    , totalPages = 1
    , itemsPerPage = 20
    , sortBy = SortByTimestamp
    , sortOrder = Descending
    , statusFilter = Nothing
    , urlSearchQuery = ""
    }


initNetworkStatus : NetworkStatus
initNetworkStatus =
    { isOnline = True
    , lastError = Nothing
    , lastSuccessfulRequest = Nothing
    }


initRetryState : RetryState
initRetryState =
    { pendingRetry = Nothing
    , retryCount = 0
    , maxRetries = 3
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


initBatchTestForm : BatchTestForm
initBatchTestForm =
    { urls = []
    , urlInput = ""
    , maxDuration = 60
    , headless = True  -- Batch tests are always headless
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
        , Parser.map BatchTestSubmission (Parser.s "batch")
        , Parser.map TestStatusPage (Parser.s "test" </> string)
        , Parser.map BatchTestStatusPage (Parser.s "batch" </> string)
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
    | UpdateLogSearch String
    | ToggleLevelFilter String
    | ToggleLogExpanded Int
    | ScrollLogs Int
    | ExportLogs String
    | FetchTestHistory
    | TestHistoryFetched (Result Http.Error (List ReportSummary))
    | ChangeSortField SortField
    | ToggleSortOrder
    | UpdateStatusFilter String
    | UpdateUrlSearch String
    | ChangeHistoryPage Int
    | NavigateToReport String
    | NetworkStatusChanged Bool
    | RetryRequest
    | DismissError
    | UpdateBatchUrlInput String
    | AddBatchUrl
    | AddQuickGameToBatch String
    | FillRandomBatchUrls
    | RandomUrlsGenerated (List Int)
    | RemoveBatchUrl Int
    | UpdateBatchMaxDuration String
    | ToggleBatchHeadless
    | SubmitBatchTest
    | BatchTestSubmitted (Result Http.Error BatchTestSubmitResponse)
    | PollBatchStatus String
    | BatchStatusUpdated (Result Http.Error BatchTestStatus)


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
                    let
                        errorString = httpErrorToString error
                        oldStatus = model.networkStatus
                        newStatus =
                            { oldStatus
                                | isOnline = not (isNetworkError error)
                                , lastError = Just errorString
                            }
                    in
                    ( { model
                        | testForm =
                            { form
                                | submitting = False
                                , submitError = Just errorString
                            }
                        , networkStatus = newStatus
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

                BatchTestStatusPage batchId ->
                    ( model, pollBatchStatus model.apiBaseUrl batchId )

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

        UpdateLogSearch query ->
            let
                logViewer =
                    model.consoleLogViewer

                updatedViewer =
                    { logViewer | searchQuery = query }
            in
            ( { model | consoleLogViewer = updatedViewer }, Cmd.none )

        ToggleLevelFilter level ->
            let
                logViewer =
                    model.consoleLogViewer

                filters =
                    logViewer.levelFilters

                updatedFilters =
                    case level of
                        "error" ->
                            { filters | showError = not filters.showError }

                        "warning" ->
                            { filters | showWarning = not filters.showWarning }

                        "info" ->
                            { filters | showInfo = not filters.showInfo }

                        "debug" ->
                            { filters | showDebug = not filters.showDebug }

                        "log" ->
                            { filters | showLog = not filters.showLog }

                        _ ->
                            filters

                updatedViewer =
                    { logViewer | levelFilters = updatedFilters }
            in
            ( { model | consoleLogViewer = updatedViewer }, Cmd.none )

        ToggleLogExpanded index ->
            let
                logViewer =
                    model.consoleLogViewer

                updatedExpanded =
                    if List.member index logViewer.expandedLogs then
                        List.filter (\i -> i /= index) logViewer.expandedLogs

                    else
                        index :: logViewer.expandedLogs

                updatedViewer =
                    { logViewer | expandedLogs = updatedExpanded }
            in
            ( { model | consoleLogViewer = updatedViewer }, Cmd.none )

        ScrollLogs offset ->
            let
                logViewer =
                    model.consoleLogViewer

                updatedViewer =
                    { logViewer | virtualScrollOffset = offset }
            in
            ( { model | consoleLogViewer = updatedViewer }, Cmd.none )

        ExportLogs format ->
            -- Export functionality will be implemented via ports or download
            ( model, Cmd.none )

        FetchTestHistory ->
            ( { model | testHistory = { initTestHistory | loading = True } }
            , fetchTestHistory model.apiBaseUrl model.testHistory
            )

        TestHistoryFetched result ->
            case result of
                Ok reports ->
                    let
                        history =
                            model.testHistory

                        updatedHistory =
                            { history
                                | reports = reports
                                , loading = False
                                , error = Nothing
                                , totalPages = ceiling (toFloat (List.length reports) / toFloat history.itemsPerPage)
                            }
                    in
                    ( { model | testHistory = updatedHistory }, Cmd.none )

                Err error ->
                    let
                        history =
                            model.testHistory

                        updatedHistory =
                            { history
                                | loading = False
                                , error = Just (httpErrorToString error)
                            }
                    in
                    ( { model | testHistory = updatedHistory }, Cmd.none )

        ChangeSortField field ->
            let
                history =
                    model.testHistory

                updatedHistory =
                    { history | sortBy = field }
            in
            ( { model | testHistory = updatedHistory }, Cmd.none )

        ToggleSortOrder ->
            let
                history =
                    model.testHistory

                newOrder =
                    case history.sortOrder of
                        Ascending ->
                            Descending

                        Descending ->
                            Ascending

                updatedHistory =
                    { history | sortOrder = newOrder }
            in
            ( { model | testHistory = updatedHistory }, Cmd.none )

        UpdateStatusFilter status ->
            let
                history =
                    model.testHistory

                newFilter =
                    if String.isEmpty status then
                        Nothing

                    else
                        Just status

                updatedHistory =
                    { history | statusFilter = newFilter, currentPage = 1 }
            in
            ( { model | testHistory = updatedHistory }, Cmd.none )

        UpdateUrlSearch query ->
            let
                history =
                    model.testHistory

                updatedHistory =
                    { history | urlSearchQuery = query, currentPage = 1 }
            in
            ( { model | testHistory = updatedHistory }, Cmd.none )

        ChangeHistoryPage page ->
            let
                history =
                    model.testHistory

                updatedHistory =
                    { history | currentPage = page }
            in
            ( { model | testHistory = updatedHistory }, Cmd.none )

        NavigateToReport reportId ->
            ( model, Nav.pushUrl model.key ("/report/" ++ reportId) )

        NetworkStatusChanged isOnline ->
            let
                oldStatus = model.networkStatus
                newStatus = { oldStatus | isOnline = isOnline }
            in
            ( { model | networkStatus = newStatus }, Cmd.none )

        RetryRequest ->
            let
                retryState = model.retryState
            in
            if retryState.retryCount < retryState.maxRetries then
                -- Retry the last failed request
                ( { model | retryState = { retryState | retryCount = retryState.retryCount + 1 } }
                , Cmd.none  -- In a real implementation, would re-execute the failed request
                )
            else
                ( model, Cmd.none )

        DismissError ->
            ( { model
                | reportError = Nothing
                , networkStatus = { isOnline = True, lastError = Nothing, lastSuccessfulRequest = model.networkStatus.lastSuccessfulRequest }
              }
            , Cmd.none
            )

        UpdateBatchUrlInput input ->
            let
                form =
                    model.batchTestForm

                updatedForm =
                    { form | urlInput = input }
            in
            ( { model | batchTestForm = updatedForm }, Cmd.none )

        AddBatchUrl ->
            let
                form =
                    model.batchTestForm

                trimmedUrl =
                    String.trim form.urlInput
            in
            if String.isEmpty trimmedUrl then
                ( model, Cmd.none )

            else if List.length form.urls >= 10 then
                let
                    updatedForm =
                        { form | validationError = Just "Maximum 10 URLs allowed" }
                in
                ( { model | batchTestForm = updatedForm }, Cmd.none )

            else if List.member trimmedUrl form.urls then
                let
                    updatedForm =
                        { form | validationError = Just "URL already added to batch" }
                in
                ( { model | batchTestForm = updatedForm }, Cmd.none )

            else
                case validateUrl trimmedUrl of
                    Just error ->
                        let
                            updatedForm =
                                { form | validationError = Just error }
                        in
                        ( { model | batchTestForm = updatedForm }, Cmd.none )

                    Nothing ->
                        let
                            updatedForm =
                                { form
                                    | urls = form.urls ++ [ trimmedUrl ]
                                    , urlInput = ""
                                    , validationError = Nothing
                                }
                        in
                        ( { model | batchTestForm = updatedForm }, Cmd.none )

        AddQuickGameToBatch url ->
            let
                form =
                    model.batchTestForm
            in
            if List.length form.urls >= 10 then
                let
                    updatedForm =
                        { form | validationError = Just "Maximum 10 URLs allowed" }
                in
                ( { model | batchTestForm = updatedForm }, Cmd.none )

            else if List.member url form.urls then
                let
                    updatedForm =
                        { form | validationError = Just "URL already added to batch" }
                in
                ( { model | batchTestForm = updatedForm }, Cmd.none )

            else
                let
                    updatedForm =
                        { form
                            | urls = form.urls ++ [ url ]
                            , validationError = Nothing
                        }
                in
                ( { model | batchTestForm = updatedForm }, Cmd.none )

        FillRandomBatchUrls ->
            let
                availableUrls =
                    List.map .url gameLibrary

                -- Generate command to select random URLs
                randomIndicesGenerator =
                    Random.list 10 (Random.int 0 (List.length availableUrls - 1))
            in
            ( model, Random.generate RandomUrlsGenerated randomIndicesGenerator )

        RandomUrlsGenerated indices ->
            let
                form =
                    model.batchTestForm

                availableUrls =
                    List.map .url gameLibrary

                -- Convert indices to URLs
                selectedUrls =
                    indices
                        |> List.filterMap (\i -> List.head (List.drop i availableUrls))
                        |> List.take 10

                updatedForm =
                    { form
                        | urls = selectedUrls
                        , validationError = Nothing
                    }
            in
            ( { model | batchTestForm = updatedForm }, Cmd.none )

        RemoveBatchUrl index ->
            let
                form =
                    model.batchTestForm

                updatedUrls =
                    List.take index form.urls ++ List.drop (index + 1) form.urls

                updatedForm =
                    { form | urls = updatedUrls, validationError = Nothing }
            in
            ( { model | batchTestForm = updatedForm }, Cmd.none )

        UpdateBatchMaxDuration durationStr ->
            let
                form =
                    model.batchTestForm

                duration =
                    String.toInt durationStr |> Maybe.withDefault 60

                updatedForm =
                    { form | maxDuration = duration }
            in
            ( { model | batchTestForm = updatedForm }, Cmd.none )

        ToggleBatchHeadless ->
            let
                form =
                    model.batchTestForm

                updatedForm =
                    { form | headless = not form.headless }
            in
            ( { model | batchTestForm = updatedForm }, Cmd.none )

        SubmitBatchTest ->
            let
                form =
                    model.batchTestForm
            in
            if List.isEmpty form.urls then
                let
                    updatedForm =
                        { form | validationError = Just "Please add at least one URL" }
                in
                ( { model | batchTestForm = updatedForm }, Cmd.none )

            else
                let
                    updatedForm =
                        { form | submitting = True, submitError = Nothing }
                in
                ( { model | batchTestForm = updatedForm }
                , submitBatchTestRequest model.apiBaseUrl form
                )

        BatchTestSubmitted (Ok response) ->
            let
                form =
                    model.batchTestForm

                updatedForm =
                    { form | submitting = False }
            in
            ( { model | batchTestForm = updatedForm }
            , Nav.pushUrl model.key ("/batch/" ++ response.batchId)
            )

        BatchTestSubmitted (Err error) ->
            let
                form =
                    model.batchTestForm

                errorMsg =
                    case error of
                        Http.BadUrl _ ->
                            "Invalid URL"

                        Http.Timeout ->
                            "Request timed out"

                        Http.NetworkError ->
                            "Network error"

                        Http.BadStatus status ->
                            "Server error: " ++ String.fromInt status

                        Http.BadBody errMsg ->
                            "Response error: " ++ errMsg

                updatedForm =
                    { form | submitting = False, submitError = Just errorMsg }
            in
            ( { model | batchTestForm = updatedForm }, Cmd.none )

        PollBatchStatus batchId ->
            ( model, pollBatchStatus model.apiBaseUrl batchId )

        BatchStatusUpdated (Ok status) ->
            -- Stop polling if batch is complete
            let
                isComplete =
                    status.status == "completed" || status.status == "completed_with_failures"
            in
            ( { model | batchTestStatus = Just status }, Cmd.none )

        BatchStatusUpdated (Err error) ->
            ( model, Cmd.none )


-- DECODER HELPERS


-- Helper for pipeline-style decoding (needed for decoders with > 8 fields)
andMap : Decode.Decoder a -> Decode.Decoder (a -> b) -> Decode.Decoder b
andMap =
    Decode.map2 (|>)


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
    , status : String
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
        (Decode.field "status" Decode.string)


pollTestStatus : String -> String -> Cmd Msg
pollTestStatus apiBaseUrl testId =
    getWithCors
        (apiBaseUrl ++ "/tests/" ++ testId)
        testStatusDecoder
        StatusUpdated


testStatusDecoder : Decode.Decoder TestStatus
testStatusDecoder =
    Decode.map4 TestStatus
        (Decode.field "testId" Decode.string)
        (Decode.field "status" Decode.string)
        (Decode.field "progress" Decode.int)
        (Decode.field "message" Decode.string)


-- BATCH TEST API


type alias BatchTestSubmitResponse =
    { batchId : String
    , testIds : List String
    , status : String
    }


submitBatchTestRequest : String -> BatchTestForm -> Cmd Msg
submitBatchTestRequest apiBaseUrl form =
    let
        body =
            Encode.object
                [ ( "urls", Encode.list Encode.string form.urls )
                , ( "maxDuration", Encode.int form.maxDuration )
                , ( "headless", Encode.bool form.headless )
                ]
    in
    postWithCors
        (apiBaseUrl ++ "/batch-tests")
        body
        batchTestSubmitResponseDecoder
        BatchTestSubmitted


batchTestSubmitResponseDecoder : Decode.Decoder BatchTestSubmitResponse
batchTestSubmitResponseDecoder =
    Decode.map3 BatchTestSubmitResponse
        (Decode.field "batchId" Decode.string)
        (Decode.field "testIds" (Decode.list Decode.string))
        (Decode.field "status" Decode.string)


pollBatchStatus : String -> String -> Cmd Msg
pollBatchStatus apiBaseUrl batchId =
    getWithCors
        (apiBaseUrl ++ "/batch-tests/" ++ batchId)
        batchStatusDecoder
        BatchStatusUpdated


batchStatusDecoder : Decode.Decoder BatchTestStatus
batchStatusDecoder =
    Decode.succeed BatchTestStatus
        |> andMap (Decode.field "batchId" Decode.string)
        |> andMap (Decode.field "status" Decode.string)
        |> andMap (Decode.field "tests" (Decode.list testStatusDecoder))
        |> andMap (Decode.field "totalTests" Decode.int)
        |> andMap (Decode.field "completedTests" Decode.int)
        |> andMap (Decode.field "failedTests" Decode.int)
        |> andMap (Decode.field "runningTests" Decode.int)
        |> andMap (Decode.field "createdAt" Decode.string)
        |> andMap (Decode.field "updatedAt" Decode.string)


fetchReport : String -> String -> Cmd Msg
fetchReport apiBaseUrl reportId =
    getWithCors
        (apiBaseUrl ++ "/reports/" ++ reportId)
        reportDecoder
        ReportFetched


fetchTestHistory : String -> TestHistoryState -> Cmd Msg
fetchTestHistory apiBaseUrl history =
    let
        queryParams =
            [ "page=" ++ String.fromInt history.currentPage
            , "limit=" ++ String.fromInt history.itemsPerPage
            ]
                |> String.join "&"
    in
    getWithCors
        (apiBaseUrl ++ "/reports?" ++ queryParams)
        (Decode.list reportSummaryDecoder)
        TestHistoryFetched


reportSummaryDecoder : Decode.Decoder ReportSummary
reportSummaryDecoder =
    Decode.map6 ReportSummary
        (Decode.field "report_id" Decode.string)
        (Decode.field "game_url" Decode.string)
        (Decode.field "timestamp" Decode.string)
        (Decode.field "status" Decode.string)
        (Decode.maybe (Decode.at [ "score", "overall_score" ] Decode.int))
        (Decode.field "duration_ms" Decode.int)


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
    Decode.map5 Evidence
        (Decode.field "screenshots" (Decode.list screenshotInfoDecoder))
        (Decode.maybe (Decode.field "video_url" Decode.string))
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
        (Decode.field "Level" Decode.string)
        (Decode.field "Message" Decode.string)
        (Decode.field "Source" Decode.string)
        (Decode.field "Timestamp" Decode.string)


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


isNetworkError : Http.Error -> Bool
isNetworkError error =
    case error of
        Http.NetworkError ->
            True

        Http.Timeout ->
            True

        _ ->
            False


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

        BatchTestStatusPage _ ->
            -- Only poll if batch is still running
            case model.batchTestStatus of
                Just status ->
                    if status.status == "completed" || status.status == "completed_with_failures" then
                        Sub.none
                    else
                        Time.every 3000 Tick

                Nothing ->
                    Time.every 3000 Tick

        _ ->
            Sub.none


-- VIEW


view : Model -> Browser.Document Msg
view model =
    { title = "DreamUp QA Agent Dashboard"
    , body =
        [ div [ class "min-h-screen bg-gray-50 flex flex-col" ]
            [ viewHeader model
            , viewContent model
            , viewFooter
            ]
        ]
    }


viewHeader : Model -> Html Msg
viewHeader model =
    header [ class "bg-slate-900 text-white shadow-lg" ]
        [ div [ class "max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4" ]
            [ div [ class "flex items-center justify-between" ]
                [ h1 [ class "text-2xl font-bold tracking-tight" ] [ text "DreamUp QA Agent" ]
                , div [ class "flex items-center gap-6" ]
                    [ nav [ class "flex gap-4" ]
                        [ a [ href "/", class "text-gray-300 hover:text-white px-3 py-2 rounded-md text-sm font-medium transition-colors" ] [ text "Home" ]
                        , a [ href "/submit", class "text-gray-300 hover:text-white px-3 py-2 rounded-md text-sm font-medium transition-colors" ] [ text "Submit Test" ]
                        , a [ href "/history", class "text-gray-300 hover:text-white px-3 py-2 rounded-md text-sm font-medium transition-colors" ] [ text "Test History" ]
                        ]
                    , viewNetworkStatus model.networkStatus
                    ]
                ]
            ]
        ]


viewNetworkStatus : NetworkStatus -> Html Msg
viewNetworkStatus status =
    if status.isOnline then
        span [ class "flex items-center gap-2 text-emerald-400 text-sm" ]
            [ span [ class "w-2 h-2 bg-emerald-400 rounded-full animate-pulse" ] []
            , text "Online"
            ]
    else
        span [ class "flex items-center gap-2 text-red-400 text-sm", title "Network connection lost" ]
            [ span [ class "w-2 h-2 bg-red-400 rounded-full" ] []
            , text "Offline"
            ]


viewContent : Model -> Html Msg
viewContent model =
    main_ [ class "flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 py-8" ]
        [ case model.route of
            Home ->
                viewHome

            TestSubmission ->
                viewTestSubmission model

            BatchTestSubmission ->
                viewBatchTestSubmission model

            TestStatusPage testId ->
                viewTestStatus model testId

            BatchTestStatusPage batchId ->
                viewBatchTestStatus model batchId

            ReportView reportId ->
                viewReportView model reportId

            TestHistory ->
                viewTestHistory model

            NotFound ->
                viewNotFound
        ]


viewHome : Html Msg
viewHome =
    div [ class "space-y-8" ]
        [ div [ class "bg-white rounded-lg shadow-md p-8 border border-gray-200" ]
            [ h2 [ class "text-3xl font-bold text-gray-900 mb-4" ] [ text "Welcome to DreamUp QA Agent" ]
            , p [ class "text-lg text-gray-600 mb-6" ] [ text "An automated QA testing system for web games. Test game functionality, performance, and compatibility with AI-powered analysis." ]
            , div [ class "flex gap-4" ]
                [ a [ href "/submit", class "inline-flex items-center px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg shadow-sm transition-colors duration-200" ] [ text "Submit New Test" ]
                , a [ href "/batch", class "inline-flex items-center px-6 py-3 bg-green-600 hover:bg-green-700 text-white font-medium rounded-lg shadow-sm transition-colors duration-200" ] [ text "Batch Test (up to 10 URLs)" ]
                , a [ href "/history", class "inline-flex items-center px-6 py-3 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg shadow-sm transition-colors duration-200" ] [ text "View Test History" ]
                ]
            ]
        , div [ class "grid grid-cols-1 md:grid-cols-3 gap-6" ]
            [ div [ class "bg-white rounded-lg shadow-md p-6 border border-gray-200" ]
                [ div [ class "text-4xl mb-3" ] [ text "🎮" ]
                , h3 [ class "text-xl font-semibold text-gray-900 mb-2" ] [ text "Automated Testing" ]
                , p [ class "text-gray-600" ] [ text "Submit game URLs for automated QA testing with browser automation." ]
                ]
            , div [ class "bg-white rounded-lg shadow-md p-6 border border-gray-200" ]
                [ div [ class "text-4xl mb-3" ] [ text "🤖" ]
                , h3 [ class "text-xl font-semibold text-gray-900 mb-2" ] [ text "AI Analysis" ]
                , p [ class "text-gray-600" ] [ text "Get AI-powered insights on game functionality, errors, and performance." ]
                ]
            , div [ class "bg-white rounded-lg shadow-md p-6 border border-gray-200" ]
                [ div [ class "text-4xl mb-3" ] [ text "📊" ]
                , h3 [ class "text-xl font-semibold text-gray-900 mb-2" ] [ text "Detailed Reports" ]
                , p [ class "text-gray-600" ] [ text "View comprehensive test reports with screenshots and console logs." ]
                ]
            ]
        ]


viewTestSubmission : Model -> Html Msg
viewTestSubmission model =
    let
        form =
            model.testForm
    in
    div [ class "space-y-8" ]
        [ div [ class "bg-white rounded-lg shadow-md p-8 border border-gray-200" ]
            [ h2 [ class "text-2xl font-bold text-gray-900 mb-2" ] [ text "Submit New Test" ]
            , p [ class "text-gray-600 mb-6" ] [ text "Enter a game URL to start automated QA testing." ]
            , Html.form [ onSubmit SubmitTest, class "space-y-6" ]
                [ div [ class "space-y-2" ]
                    [ label [ for "game-url", class "block text-sm font-medium text-gray-700" ] [ text "Game URL" ]
                    , input
                        [ type_ "text"
                        , id "game-url"
                        , placeholder "https://example.com/game"
                        , value form.gameUrl
                        , onInput UpdateGameUrl
                        , disabled form.submitting
                        , class "w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all disabled:bg-gray-100 disabled:cursor-not-allowed"
                        ]
                        []
                    , case form.validationError of
                        Just error ->
                            div [ class "text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-4 py-2" ] [ text error ]

                        Nothing ->
                            text ""
                    ]
                , div [ class "space-y-2" ]
                    [ label [ for "max-duration", class "block text-sm font-medium text-gray-700" ]
                        [ text ("Max Duration: " ++ String.fromInt form.maxDuration ++ "s") ]
                    , input
                        [ type_ "range"
                        , id "max-duration"
                        , Html.Attributes.min "60"
                        , Html.Attributes.max "300"
                        , value (String.fromInt form.maxDuration)
                        , onInput UpdateMaxDuration
                        , disabled form.submitting
                        , class "w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-blue-600 disabled:cursor-not-allowed"
                        ]
                        []
                    , p [ class "text-sm text-gray-500" ] [ text "Maximum time allowed for the test (60-300 seconds)" ]
                    ]
                , div [ class "flex items-center" ]
                    [ label [ class "flex items-center cursor-pointer" ]
                        [ input
                            [ type_ "checkbox"
                            , checked form.headless
                            , onClick ToggleHeadless
                            , disabled form.submitting
                            , class "w-4 h-4 text-blue-600 bg-gray-100 border-gray-300 rounded focus:ring-blue-500 focus:ring-2 disabled:cursor-not-allowed"
                            ]
                            []
                        , span [ class "ml-2 text-sm font-medium text-gray-700" ] [ text "Run in headless mode (no visible browser)" ]
                        ]
                    ]
                , case form.submitError of
                    Just error ->
                        div [ class "text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-4 py-3" ] [ text error ]

                    Nothing ->
                        text ""
                , div [ class "flex gap-3 pt-4" ]
                    [ button
                        [ type_ "submit"
                        , class "px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg shadow-sm transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
                        , disabled (form.submitting || String.isEmpty form.gameUrl)
                        ]
                        [ text
                            (if form.submitting then
                                "Submitting..."

                             else
                                "Start Test"
                            )
                        ]
                    , a [ href "/", class "px-6 py-3 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg shadow-sm transition-colors" ] [ text "Cancel" ]
                    ]
                ]
            ]
        , div [ class "bg-white rounded-lg shadow-md p-8 border border-gray-200" ]
            [ h3 [ class "text-xl font-bold text-gray-900 mb-2" ] [ text "Example Games (Click to Test)" ]
            , p [ class "text-sm text-gray-600 mb-6" ] [ text "These games have been tested and work well with our system:" ]
            , div [ class "grid grid-cols-1 md:grid-cols-3 gap-4" ]
                (List.map viewExampleGame gameLibrary)
            ]
        ]


viewExampleGame : ExampleGame -> Html Msg
viewExampleGame game =
    let
        badgeColor =
            case game.badgeType of
                "success" -> "bg-green-100 text-green-800 border-green-200"
                "warning" -> "bg-yellow-100 text-yellow-800 border-yellow-200"
                _ -> "bg-blue-100 text-blue-800 border-blue-200"
    in
    div
        [ class "border border-gray-200 rounded-lg hover:shadow-lg transition-shadow cursor-pointer bg-white overflow-hidden"
        , onClick (UpdateGameUrl game.url)
        ]
        [ div [ class "p-4 space-y-3" ]
            [ div [ class "flex items-start justify-between" ]
                [ h4 [ class "font-semibold text-gray-900 text-lg" ] [ text game.title ]
                , span [ class ("px-2 py-1 text-xs font-medium rounded border " ++ badgeColor) ] [ text game.score ]
                ]
            , p [ class "text-sm text-gray-600" ] [ text game.description ]
            , code [ class "block text-xs text-gray-500 bg-gray-50 px-2 py-1 rounded border border-gray-200 truncate" ] [ text game.url ]
            , button
                [ class "w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-md transition-colors"
                , type_ "button"
                , onClick (UpdateGameUrl game.url)
                ]
                [ text "Use This URL" ]
            ]
        ]


viewBatchTestSubmission : Model -> Html Msg
viewBatchTestSubmission model =
    let
        form =
            model.batchTestForm
    in
    div [ class "space-y-8" ]
        [ div [ class "bg-white rounded-lg shadow-md p-8 border border-gray-200" ]
            [ h2 [ class "text-2xl font-bold text-gray-900 mb-2" ] [ text "Batch Test Submission" ]
            , p [ class "text-gray-600 mb-6" ] [ text "Test up to 10 game URLs concurrently. Add URLs one at a time below." ]
            , div [ class "space-y-6" ]
                [ -- URL input section
                  div [ class "space-y-2" ]
                    [ label [ for "url-input", class "block text-sm font-medium text-gray-700" ]
                        [ text ("Add URL (" ++ String.fromInt (List.length form.urls) ++ "/10)") ]
                    , div [ class "flex gap-2" ]
                        [ input
                            [ type_ "text"
                            , id "url-input"
                            , placeholder "https://example.com/game"
                            , value form.urlInput
                            , onInput UpdateBatchUrlInput
                            , disabled form.submitting
                            , class "flex-1 px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent transition-all disabled:bg-gray-100 disabled:cursor-not-allowed"
                            ]
                            []
                        , button
                            [ onClick AddBatchUrl
                            , disabled (form.submitting || String.isEmpty (String.trim form.urlInput) || List.length form.urls >= 10)
                            , class "px-6 py-3 bg-green-600 hover:bg-green-700 text-white font-medium rounded-lg shadow-sm transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
                            ]
                            [ text "Add" ]
                        ]
                    , case form.validationError of
                        Just error ->
                            div [ class "text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-4 py-2" ] [ text error ]

                        Nothing ->
                            text ""
                    ]

                -- Quick-add games section
                , div [ class "space-y-3" ]
                    [ div [ class "flex items-center justify-between" ]
                        [ label [ class "block text-sm font-medium text-gray-700" ] [ text "Quick Add Games:" ]
                        , button
                            [ onClick FillRandomBatchUrls
                            , disabled form.submitting
                            , class "px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white text-sm font-medium rounded-lg shadow-sm transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
                            ]
                            [ text "🎲 Fill Random (10)" ]
                        ]
                    , div [ class "grid grid-cols-2 md:grid-cols-5 gap-2" ]
                        (List.map
                            (\game ->
                                button
                                    [ onClick (AddQuickGameToBatch game.url)
                                    , disabled (form.submitting || List.length form.urls >= 10)
                                    , class "px-3 py-2 bg-blue-50 hover:bg-blue-100 text-blue-700 text-xs font-medium rounded-md transition-colors border border-blue-200 disabled:opacity-50 disabled:cursor-not-allowed"
                                    , title game.url
                                    ]
                                    [ text ("+ " ++ game.title) ]
                            )
                            gameLibrary
                        )
                    ]

                -- URL list
                , if List.isEmpty form.urls then
                    div [ class "text-sm text-gray-500 italic p-4 bg-gray-50 rounded-lg border border-gray-200" ]
                        [ text "No URLs added yet. Add at least one URL to start batch testing." ]
                  else
                    div [ class "space-y-2" ]
                        [ label [ class "block text-sm font-medium text-gray-700" ] [ text "URLs to Test:" ]
                        , div [ class "space-y-2" ]
                            (List.indexedMap
                                (\index url ->
                                    div [ class "flex items-center gap-2 p-3 bg-gray-50 rounded-lg border border-gray-200" ]
                                        [ span [ class "flex-1 text-sm text-gray-700 font-mono truncate" ] [ text url ]
                                        , button
                                            [ onClick (RemoveBatchUrl index)
                                            , disabled form.submitting
                                            , class "px-3 py-1 text-sm bg-red-100 hover:bg-red-200 text-red-700 rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                                            ]
                                            [ text "Remove" ]
                                        ]
                                )
                                form.urls
                            )
                        ]

                -- Settings
                , div [ class "space-y-2" ]
                    [ label [ for "batch-max-duration", class "block text-sm font-medium text-gray-700" ]
                        [ text ("Max Duration per Test: " ++ String.fromInt form.maxDuration ++ "s") ]
                    , input
                        [ type_ "range"
                        , id "batch-max-duration"
                        , Html.Attributes.min "60"
                        , Html.Attributes.max "300"
                        , value (String.fromInt form.maxDuration)
                        , onInput UpdateBatchMaxDuration
                        , disabled form.submitting
                        , class "w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-green-600 disabled:cursor-not-allowed"
                        ]
                        []
                    , p [ class "text-sm text-gray-500" ] [ text "Maximum time allowed per test (60-300 seconds)" ]
                    ]

                -- Note: Batch tests always run in headless mode for better performance

                , case form.submitError of
                    Just error ->
                        div [ class "text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-4 py-3" ] [ text error ]

                    Nothing ->
                        text ""

                , div [ class "flex gap-3 pt-4" ]
                    [ button
                        [ onClick SubmitBatchTest
                        , class "px-6 py-3 bg-green-600 hover:bg-green-700 text-white font-medium rounded-lg shadow-sm transition-colors disabled:bg-gray-300 disabled:cursor-not-allowed"
                        , disabled (form.submitting || List.isEmpty form.urls)
                        ]
                        [ text
                            (if form.submitting then
                                "Submitting..."
                             else
                                "Start Batch Test"
                            )
                        ]
                    , a [ href "/", class "px-6 py-3 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg shadow-sm transition-colors" ] [ text "Cancel" ]
                    ]
                ]
            ]
        ]


viewBatchTestStatus : Model -> String -> Html Msg
viewBatchTestStatus model batchId =
    div [ class "max-w-6xl mx-auto space-y-6" ]
        [ div [ class "bg-white rounded-lg shadow-md p-8 border border-gray-200" ]
            [ h2 [ class "text-2xl font-bold text-gray-900 mb-2" ] [ text "Batch Test Status" ]
            , p [ class "text-sm text-gray-600 mb-4" ] [ text ("Batch ID: " ++ batchId) ]
            , case model.batchTestStatus of
                Nothing ->
                    div [ class "flex items-center justify-center py-12" ]
                        [ div [ class "animate-spin rounded-full h-12 w-12 border-b-2 border-green-600" ] []
                        , p [ class "ml-4 text-gray-600" ] [ text "Loading batch status..." ]
                        ]

                Just status ->
                    div [ class "space-y-6" ]
                        [ -- Overall batch status with statistics
                          div [ class ("p-4 rounded-lg border-2 " ++ batchStatusColor status.status) ]
                            [ div [ class "flex items-center justify-between" ]
                                [ div []
                                    [ p [ class "text-sm font-medium text-gray-700" ] [ text "Batch Status" ]
                                    , p [ class "text-2xl font-bold" ] [ text (batchStatusText status.status) ]
                                    ]
                                , div [ class "text-right space-y-1" ]
                                    [ p [ class "text-sm text-gray-600" ] [ text ("Total: " ++ String.fromInt status.totalTests) ]
                                    , p [ class "text-sm text-green-600 font-medium" ] [ text ("✓ Completed: " ++ String.fromInt status.completedTests) ]
                                    , if status.failedTests > 0 then
                                        p [ class "text-sm text-red-600 font-medium" ] [ text ("✗ Failed: " ++ String.fromInt status.failedTests) ]
                                      else
                                        text ""
                                    , if status.runningTests > 0 then
                                        p [ class "text-sm text-blue-600 font-medium" ] [ text ("⟳ Running: " ++ String.fromInt status.runningTests) ]
                                      else
                                        text ""
                                    ]
                                ]
                            ]

                        -- Individual test statuses
                        , div [ class "space-y-4" ]
                            [ h3 [ class "text-lg font-semibold text-gray-900" ] [ text "Individual Test Results" ]
                            , div [ class "grid gap-4" ]
                                (List.map viewBatchTestItem status.tests)
                            ]
                        ]
            ]
        ]


viewBatchTestItem : TestStatus -> Html Msg
viewBatchTestItem test =
    let
        statusColor =
            case test.status of
                "completed" ->
                    "border-green-500 bg-green-50"

                "failed" ->
                    "border-red-500 bg-red-50"

                "running" ->
                    "border-blue-500 bg-blue-50"

                _ ->
                    "border-gray-300 bg-gray-50"
    in
    div [ class ("p-4 rounded-lg border-2 " ++ statusColor) ]
        [ div [ class "flex items-center justify-between mb-2" ]
            [ div [ class "flex-1" ]
                [ p [ class "text-sm font-medium text-gray-700" ] [ text "Test ID" ]
                , p [ class "text-xs text-gray-600 font-mono" ] [ text test.testId ]
                ]
            , div [ class "flex gap-2" ]
                [ if test.status == "completed" then
                    a
                        [ href ("/report/" ++ test.testId)
                        , class "px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg shadow-sm transition-colors"
                        ]
                        [ text "View Report" ]
                  else
                    text ""
                ]
            ]
        , div [ class "mt-2" ]
            [ p [ class "text-sm text-gray-700" ] [ text test.message ]
            , if test.status == "running" then
                div [ class "mt-2" ]
                    [ div [ class "w-full bg-gray-200 rounded-full h-2" ]
                        [ div
                            [ class "bg-blue-600 h-2 rounded-full transition-all duration-300"
                            , style "width" (String.fromInt test.progress ++ "%")
                            ]
                            []
                        ]
                    , p [ class "text-xs text-gray-600 mt-1" ] [ text (String.fromInt test.progress ++ "% complete") ]
                    ]
              else
                text ""
            ]
        ]


batchStatusColor : String -> String
batchStatusColor status =
    case status of
        "completed" ->
            "border-green-500 bg-green-50"

        "completed_with_failures" ->
            "border-yellow-500 bg-yellow-50"

        "running" ->
            "border-blue-500 bg-blue-50"

        _ ->
            "border-gray-300 bg-gray-50"


batchStatusText : String -> String
batchStatusText status =
    case status of
        "completed" ->
            "✓ All Tests Completed"

        "completed_with_failures" ->
            "⚠ Completed with Failures"

        "running" ->
            "⟳ Tests Running..."

        _ ->
            status


countCompletedTests : List TestStatus -> Int
countCompletedTests tests =
    List.filter (\t -> t.status == "completed" || t.status == "failed") tests
        |> List.length


viewTestStatus : Model -> String -> Html Msg
viewTestStatus model testId =
    div [ class "max-w-4xl mx-auto space-y-6" ]
        [ div [ class "bg-white rounded-lg shadow-md p-8 border border-gray-200" ]
            [ h2 [ class "text-2xl font-bold text-gray-900 mb-6" ] [ text "Test Execution Status" ]
            , case model.testStatus of
                Just status ->
                    viewStatusDetails status

                Nothing ->
                    div [ class "flex flex-col items-center justify-center py-12" ]
                        [ div [ class "animate-spin rounded-full h-16 w-16 border-b-2 border-blue-600 mb-4" ] []
                        , p [ class "text-gray-600" ] [ text "Loading test status..." ]
                        ]
            ]
        ]


viewStatusDetails : TestStatus -> Html Msg
viewStatusDetails status =
    let
        (badgeColor, badgeText) = statusBadge status.status
    in
    div [ class "space-y-6" ]
        [ div [ class "flex items-center justify-between" ]
            [ div [ class ("inline-flex items-center px-4 py-2 rounded-full text-sm font-semibold " ++ badgeColor) ]
                [ text badgeText ]
            , p [ class "text-sm text-gray-500" ] [ text ("Test ID: " ++ status.testId) ]
            ]
        , div [ class "bg-gray-50 rounded-lg p-4 border border-gray-200" ]
            [ div [ class "flex items-center justify-between mb-2" ]
                [ span [ class "text-sm font-medium text-gray-700" ] [ text "Current Status:" ]
                , span [ class "text-sm text-gray-900 font-semibold" ] [ text status.message ]
                ]
            ]
        , div [ class "space-y-2" ]
            [ div [ class "flex items-center justify-between text-sm" ]
                [ span [ class "font-medium text-gray-700" ] [ text "Progress" ]
                , span [ class "font-semibold text-gray-900" ] [ text (String.fromInt status.progress ++ "%") ]
                ]
            , div [ class "w-full bg-gray-200 rounded-full h-3 overflow-hidden" ]
                [ div
                    [ class "bg-blue-600 h-3 rounded-full transition-all duration-300 ease-out"
                    , style "width" (String.fromInt status.progress ++ "%")
                    ]
                    []
                ]
            ]
        , if status.status == "failed" then
            div [ class "bg-red-50 border border-red-200 rounded-lg p-4" ]
                [ div [ class "flex items-start" ]
                    [ div [ class "flex-shrink-0" ]
                        [ span [ class "text-red-600 text-xl" ] [ text "⚠" ]
                        ]
                    , div [ class "ml-3" ]
                        [ h3 [ class "text-sm font-medium text-red-800 mb-1" ] [ text "Test Failed" ]
                        , p [ class "text-sm text-red-700" ] [ text status.message ]
                        ]
                    ]
                ]

          else
            text ""
        , if status.status == "completed" then
            div [ class "flex gap-3 pt-4" ]
                [ a [ href ("/report/" ++ status.testId), class "flex-1 px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg shadow-sm transition-colors text-center" ]
                    [ text "View Full Report" ]
                , a [ href "/", class "px-6 py-3 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg shadow-sm transition-colors" ] [ text "Home" ]
                ]

          else if status.status == "failed" then
            div [ class "flex gap-3 pt-4" ]
                [ a [ href "/submit", class "flex-1 px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg shadow-sm transition-colors text-center" ] [ text "Try Again" ]
                , a [ href "/", class "px-6 py-3 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg shadow-sm transition-colors" ] [ text "Home" ]
                ]

          else
            div [ class "bg-blue-50 border border-blue-200 rounded-lg p-4" ]
                [ div [ class "flex items-center" ]
                    [ div [ class "animate-pulse flex-shrink-0" ]
                        [ span [ class "text-blue-600 text-xl" ] [ text "●" ]
                        ]
                    , div [ class "ml-3" ]
                        [ p [ class "text-sm text-blue-800 font-medium" ] [ text "Test is running... This page will update automatically." ]
                        ]
                    ]
                ]
        ]


statusBadge : String -> (String, String)
statusBadge status =
    case status of
        "completed" ->
            ("bg-green-100 text-green-800 border border-green-200", "✓ COMPLETED")

        "failed" ->
            ("bg-red-100 text-red-800 border border-red-200", "✗ FAILED")

        "running" ->
            ("bg-blue-100 text-blue-800 border border-blue-200", "● RUNNING")

        _ ->
            ("bg-gray-100 text-gray-800 border border-gray-200", "○ PENDING")


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
    div [ class "max-w-7xl mx-auto px-4 py-8 space-y-8" ]
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

        -- Video and Screenshots
        , viewMediaSection model report.evidence

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
    div [ class "bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6 space-y-4" ]
        [ div [ class "flex items-center justify-between" ]
            [ h2 [ class "text-3xl font-bold text-gray-900 dark:text-white" ] [ text "Test Report" ]
            , div [ class ("px-4 py-2 rounded-full font-semibold text-sm " ++ statusBadgeClass report.summary.status) ]
                [ text (String.toUpper report.summary.status) ]
            ]
        , div [ class "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" ]
            [ div [ class "space-y-1" ]
                [ span [ class "text-sm text-gray-500 dark:text-gray-400" ] [ text "Game URL:" ]
                , a [ href report.gameUrl, class "text-blue-600 dark:text-blue-400 hover:underline break-all block" ] [ text report.gameUrl ]
                ]
            , div [ class "space-y-1" ]
                [ span [ class "text-sm text-gray-500 dark:text-gray-400" ] [ text "Report ID:" ]
                , span [ class "text-gray-900 dark:text-white font-mono text-sm" ] [ text report.reportId ]
                ]
            , div [ class "space-y-1" ]
                [ span [ class "text-sm text-gray-500 dark:text-gray-400" ] [ text "Timestamp:" ]
                , span [ class "text-gray-900 dark:text-white" ] [ text report.timestamp ]
                ]
            , div [ class "space-y-1" ]
                [ span [ class "text-sm text-gray-500 dark:text-gray-400" ] [ text "Duration:" ]
                , span [ class "text-gray-900 dark:text-white" ] [ text (formatDuration report.duration) ]
                ]
            , case report.score of
                Just score ->
                    div [ class "space-y-1" ]
                        [ span [ class "text-sm text-gray-500 dark:text-gray-400" ] [ text "Overall Score:" ]
                        , span [ class ("text-2xl font-bold " ++ scoreColorClass score.overallScore) ]
                            [ text (String.fromInt score.overallScore ++ "/100") ]
                        ]

                Nothing ->
                    text ""
            ]
        ]


viewReportSummary : Report -> Html Msg
viewReportSummary report =
    div [ class "bg-white dark:bg-gray-800 rounded-lg shadow p-6" ]
        [ h3 [ class "text-xl font-bold text-gray-900 dark:text-white mb-4" ] [ text "Summary" ]
        , div [ class "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" ]
            [ if List.isEmpty report.summary.passedChecks |> not then
                div [ class "bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4" ]
                    [ h4 [ class "text-lg font-semibold text-green-900 dark:text-green-100 mb-2" ]
                        [ text ("✓ Passed (" ++ String.fromInt (List.length report.summary.passedChecks) ++ ")") ]
                    , ul [ class "space-y-1" ]
                        (List.map (\check ->
                            li [ class "text-sm text-green-800 dark:text-green-200" ] [ text check ]
                        ) report.summary.passedChecks)
                    ]

              else
                text ""
            , if List.isEmpty report.summary.failedChecks |> not then
                div [ class "bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4" ]
                    [ h4 [ class "text-lg font-semibold text-red-900 dark:text-red-100 mb-2" ]
                        [ text ("✗ Failed (" ++ String.fromInt (List.length report.summary.failedChecks) ++ ")") ]
                    , ul [ class "space-y-1" ]
                        (List.map (\check ->
                            li [ class "text-sm text-red-800 dark:text-red-200" ] [ text check ]
                        ) report.summary.failedChecks)
                    ]

              else
                text ""
            , if List.isEmpty report.summary.criticalIssues |> not then
                div [ class "bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4" ]
                    [ h4 [ class "text-lg font-semibold text-yellow-900 dark:text-yellow-100 mb-2" ]
                        [ text ("⚠ Critical Issues (" ++ String.fromInt (List.length report.summary.criticalIssues) ++ ")") ]
                    , ul [ class "space-y-1" ]
                        (List.map (\issue ->
                            li [ class "text-sm text-yellow-800 dark:text-yellow-200" ] [ text issue ]
                        ) report.summary.criticalIssues)
                    ]

              else
                text ""
            ]
        ]


{-| Subtask 2: Metrics Visualization with Progress Bars -}
viewMetrics : PlayabilityScore -> Html Msg
viewMetrics score =
    div [ class "bg-white dark:bg-gray-800 rounded-lg shadow p-6" ]
        [ h3 [ class "text-xl font-bold text-gray-900 dark:text-white mb-4" ] [ text "Evaluation Metrics" ]
        , div [ class "grid grid-cols-1 md:grid-cols-2 gap-4" ]
            [ viewMetricBar "Overall Score" score.overallScore
            , viewMetricBar "Interactivity" score.interactivityScore
            , viewMetricBar "Visual Quality" score.visualQuality
            , viewMetricBar "Error Severity" (100 - score.errorSeverity) -- Invert so higher is better
            , div [ class "col-span-1 md:col-span-2 flex items-center justify-between bg-gray-50 dark:bg-gray-700 rounded-lg p-4" ]
                [ span [ class "text-sm font-medium text-gray-700 dark:text-gray-300" ] [ text "Loads Correctly" ]
                , span [ class ("text-lg font-semibold " ++ if score.loadsCorrectly then "text-green-600 dark:text-green-400" else "text-red-600 dark:text-red-400") ]
                    [ text (if score.loadsCorrectly then "✓ Yes" else "✗ No") ]
                ]
            ]
        ]


viewMetricBar : String -> Int -> Html Msg
viewMetricBar label value =
    div [ class "space-y-2" ]
        [ div [ class "flex items-center justify-between" ]
            [ span [ class "text-sm font-medium text-gray-700 dark:text-gray-300" ] [ text label ]
            , span [ class ("text-sm font-bold " ++ scoreColorClass value) ]
                [ text (String.fromInt value ++ "/100") ]
            ]
        , div [ class "w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5" ]
            [ div
                [ class ("h-2.5 rounded-full transition-all duration-300 " ++
                    if value >= 80 then "bg-green-600 dark:bg-green-500"
                    else if value >= 60 then "bg-yellow-500 dark:bg-yellow-400"
                    else "bg-red-600 dark:bg-red-500")
                , style "width" (String.fromInt value ++ "%")
                ]
                []
            ]
        ]


{-| Subtask 3: Collapsible Issues and Recommendations -}
viewCollapsibleSection : Bool -> SectionType -> String -> Html Msg -> Html Msg
viewCollapsibleSection isExpanded sectionType title content =
    div [ class "bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden" ]
        [ div
            [ class "flex items-center justify-between p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            , onClick (ToggleSection sectionType)
            ]
            [ h3 [ class "text-lg font-semibold text-gray-900 dark:text-white" ] [ text title ]
            , span [ class "text-gray-500 dark:text-gray-400 text-xl" ]
                [ text (if isExpanded then "▼" else "▶") ]
            ]
        , if isExpanded then
            div [ class "px-4 pb-4 border-t border-gray-200 dark:border-gray-700" ] [ content ]

          else
            text ""
        ]


viewIssues : List String -> Html Msg
viewIssues issues =
    if List.isEmpty issues then
        div [ class "text-center py-8 text-gray-500 dark:text-gray-400" ] [ text "No issues found!" ]

    else
        ul [ class "space-y-2 pt-4" ]
            (List.map (\issue ->
                li [ class "flex items-start" ]
                    [ span [ class "text-red-500 dark:text-red-400 mr-2" ] [ text "•" ]
                    , span [ class "text-gray-700 dark:text-gray-300" ] [ text issue ]
                    ]
            ) issues)


viewRecommendations : List String -> Html Msg
viewRecommendations recommendations =
    if List.isEmpty recommendations then
        div [ class "text-center py-8 text-gray-500 dark:text-gray-400" ] [ text "No recommendations at this time." ]

    else
        ul [ class "space-y-2 pt-4" ]
            (List.map (\rec ->
                li [ class "flex items-start" ]
                    [ span [ class "text-blue-500 dark:text-blue-400 mr-2" ] [ text "→" ]
                    , span [ class "text-gray-700 dark:text-gray-300" ] [ text rec ]
                    ]
            ) recommendations)


viewReasoning : String -> Html Msg
viewReasoning reasoning =
    div [ class "pt-4" ]
        [ p [ class "text-gray-700 dark:text-gray-300 leading-relaxed whitespace-pre-wrap" ] [ text reasoning ] ]


{-| Task 6: Console Log Viewer with Filtering, Search, Virtual Scrolling, and Export -}
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


{-| Enhanced Console Log Viewer with all Task 6 features -}
viewConsoleLogViewer : Model -> Evidence -> Html Msg
viewConsoleLogViewer model evidence =
    let
        viewer =
            model.consoleLogViewer

        -- Subtask 2: Filter and Search
        filteredLogs =
            evidence.consoleLogs
                |> filterLogsByLevel viewer.levelFilters
                |> filterLogsBySearch viewer.searchQuery

        -- Subtask 3: Virtual Scrolling
        visibleLogs =
            filteredLogs
                |> List.drop viewer.virtualScrollOffset
                |> List.take viewer.virtualScrollLimit

        totalFiltered =
            List.length filteredLogs
    in
    div [ class "console-log-viewer" ]
        [ -- Subtask 2: Filter Controls
          viewLogFilters viewer.levelFilters evidence.logSummary
        , -- Subtask 2: Search Bar
          viewLogSearch viewer.searchQuery
        , -- Subtask 4: Export Button
          viewLogExportControls
        , -- Display Summary
          div [ class "log-display-info" ]
            [ text ("Showing " ++ String.fromInt (List.length visibleLogs) ++ " of " ++ String.fromInt totalFiltered ++ " logs")
            ]
        , -- Subtask 1 & 3: Log Display with Virtual Scrolling
          if List.isEmpty filteredLogs then
            div [ class "empty-state" ] [ text "No logs match current filters." ]

          else
            div [ class "console-logs-container" ]
                (visibleLogs
                    |> List.indexedMap (\idx log -> viewConsoleLogEnhanced (idx + viewer.virtualScrollOffset) log viewer.expandedLogs)
                )
        , -- Virtual Scroll Controls
          if totalFiltered > viewer.virtualScrollLimit then
            viewVirtualScrollControls viewer.virtualScrollOffset totalFiltered viewer.virtualScrollLimit

          else
            text ""
        ]


{-| Subtask 2: Level Filters -}
viewLogFilters : LevelFilters -> LogSummary -> Html Msg
viewLogFilters filters summary =
    div [ class "log-filters" ]
        [ h4 [] [ text "Filter by Level" ]
        , div [ class "filter-buttons" ]
            [ viewFilterButton "error" filters.showError summary.errors
            , viewFilterButton "warning" filters.showWarning summary.warnings
            , viewFilterButton "info" filters.showInfo summary.info
            , viewFilterButton "debug" filters.showDebug summary.debug
            , viewFilterButton "log" filters.showLog (summary.total - summary.errors - summary.warnings - summary.info - summary.debug)
            ]
        ]


viewFilterButton : String -> Bool -> Int -> Html Msg
viewFilterButton level isActive count =
    button
        [ class ("filter-btn filter-" ++ level ++ (if isActive then " active" else ""))
        , onClick (ToggleLevelFilter level)
        ]
        [ text (String.toUpper level ++ " (" ++ String.fromInt count ++ ")") ]


{-| Subtask 2: Search Bar -}
viewLogSearch : String -> Html Msg
viewLogSearch query =
    div [ class "log-search" ]
        [ h4 [] [ text "Search Logs" ]
        , input
            [ type_ "text"
            , placeholder "Search messages, sources..."
            , value query
            , onInput UpdateLogSearch
            , class "search-input"
            ]
            []
        ]


{-| Subtask 4: Export Controls -}
viewLogExportControls : Html Msg
viewLogExportControls =
    div [ class "log-export-controls" ]
        [ h4 [] [ text "Export" ]
        , div [ class "export-buttons" ]
            [ button
                [ class "export-btn"
                , onClick (ExportLogs "json")
                ]
                [ text "📥 Export as JSON" ]
            , button
                [ class "export-btn"
                , onClick (ExportLogs "text")
                ]
                [ text "📄 Export as Text" ]
            ]
        ]


{-| Subtask 1: Enhanced Log Display with Timestamps and Expandable Details -}
viewConsoleLogEnhanced : Int -> ConsoleLog -> List Int -> Html Msg
viewConsoleLogEnhanced index log expandedLogs =
    let
        isExpanded =
            List.member index expandedLogs

        levelBadgeClass =
            "log-level-badge level-" ++ log.level
    in
    div [ class ("console-log-enhanced log-" ++ log.level ++ (if isExpanded then " expanded" else "")) ]
        [ div [ class "log-header", onClick (ToggleLogExpanded index) ]
            [ span [ class "log-timestamp" ] [ text log.timestamp ]
            , span [ class levelBadgeClass ] [ text (String.toUpper log.level) ]
            , span [ class "log-message-preview" ] [ text (truncateMessage log.message 100) ]
            , span [ class "log-expand-icon" ] [ text (if isExpanded then "▼" else "▶") ]
            ]
        , if isExpanded then
            div [ class "log-details" ]
                [ div [ class "log-detail-row" ]
                    [ span [ class "log-detail-label" ] [ text "Message:" ]
                    , span [ class "log-detail-value" ] [ text log.message ]
                    ]
                , div [ class "log-detail-row" ]
                    [ span [ class "log-detail-label" ] [ text "Source:" ]
                    , span [ class "log-detail-value log-source" ] [ text log.source ]
                    ]
                ]

          else
            text ""
        ]


{-| Subtask 3: Virtual Scroll Controls -}
viewVirtualScrollControls : Int -> Int -> Int -> Html Msg
viewVirtualScrollControls offset total limit =
    let
        canScrollUp =
            offset > 0

        canScrollDown =
            offset + limit < total

        currentPage =
            (offset // limit) + 1

        totalPages =
            ceiling (toFloat total / toFloat limit)
    in
    div [ class "virtual-scroll-controls" ]
        [ button
            [ class "scroll-btn"
            , onClick (ScrollLogs (Basics.max 0 (offset - limit)))
            , disabled (not canScrollUp)
            ]
            [ text "◀ Previous" ]
        , span [ class "scroll-info" ]
            [ text ("Page " ++ String.fromInt currentPage ++ " of " ++ String.fromInt totalPages) ]
        , button
            [ class "scroll-btn"
            , onClick (ScrollLogs (offset + limit))
            , disabled (not canScrollDown)
            ]
            [ text "Next ▶" ]
        ]


{-| Legacy console log view (simple) with Tailwind styling -}
viewConsoleLog : ConsoleLog -> Html Msg
viewConsoleLog log =
    let
        levelColor =
            case log.level of
                "error" -> "text-red-600 bg-red-50 border-red-200"
                "warning" -> "text-yellow-700 bg-yellow-50 border-yellow-200"
                "info" -> "text-blue-600 bg-blue-50 border-blue-200"
                _ -> "text-gray-600 bg-gray-50 border-gray-200"
    in
    div [ class ("flex items-start gap-3 p-3 rounded border mb-2 " ++ levelColor) ]
        [ span [ class "font-bold text-xs uppercase flex-shrink-0" ] [ text log.level ]
        , div [ class "flex-1 space-y-1" ]
            [ div [ class "text-sm font-mono break-words" ] [ text log.message ]
            , div [ class "text-xs opacity-60" ] [ text log.source ]
            ]
        ]


{-| Helper: Filter logs by level -}
filterLogsByLevel : LevelFilters -> List ConsoleLog -> List ConsoleLog
filterLogsByLevel filters logs =
    logs
        |> List.filter
            (\log ->
                case log.level of
                    "error" ->
                        filters.showError

                    "warning" ->
                        filters.showWarning

                    "info" ->
                        filters.showInfo

                    "debug" ->
                        filters.showDebug

                    "log" ->
                        filters.showLog

                    _ ->
                        True
            )


{-| Helper: Filter logs by search query -}
filterLogsBySearch : String -> List ConsoleLog -> List ConsoleLog
filterLogsBySearch query logs =
    if String.isEmpty query then
        logs

    else
        let
            lowerQuery =
                String.toLower query
        in
        logs
            |> List.filter
                (\log ->
                    String.contains lowerQuery (String.toLower log.message)
                        || String.contains lowerQuery (String.toLower log.source)
                )


{-| Helper: Truncate message for preview -}
truncateMessage : String -> Int -> String
truncateMessage message maxLength =
    if String.length message > maxLength then
        String.left maxLength message ++ "..."

    else
        message


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


{-| Media Section: Video Player + Screenshot Carousel -}
viewMediaSection : Model -> Evidence -> Html Msg
viewMediaSection model evidence =
    div [ class "bg-white dark:bg-gray-800 rounded-lg shadow p-6 space-y-6" ]
        [ h3 [ class "text-xl font-bold text-gray-900 dark:text-white" ] [ text "Gameplay Recording" ]

        -- Video Player
        , case evidence.videoUrl of
            Just videoUrl ->
                div [ class "space-y-4" ]
                    [ video
                        [ class "w-full rounded-lg"
                        , Html.Attributes.attribute "controls" ""
                        , Html.Attributes.attribute "preload" "metadata"
                        ]
                        [ Html.Attributes.attribute "src" videoUrl |> (\_ -> source [ Html.Attributes.attribute "src" videoUrl, Html.Attributes.attribute "type" "video/mp4" ] [])
                        , text "Your browser does not support the video tag."
                        ]
                    ]

            Nothing ->
                text ""

        -- Screenshots Carousel
        , if List.isEmpty evidence.screenshots |> not then
            div [ class "space-y-4" ]
                [ h4 [ class "text-lg font-semibold text-gray-900 dark:text-white" ]
                    [ text ("Screenshots (" ++ String.fromInt (List.length evidence.screenshots) ++ ")") ]
                , viewScreenshotCarousel model evidence.screenshots
                ]

          else
            text ""
        ]


viewScreenshotCarousel : Model -> List ScreenshotInfo -> Html Msg
viewScreenshotCarousel model screenshots =
    let
        currentIndex =
            model.screenshotViewer.currentIndex

        currentScreenshot =
            List.drop currentIndex screenshots
                |> List.head

        -- Convert filepath to API URL
        screenshotUrl : ScreenshotInfo -> String
        screenshotUrl screenshot =
            case screenshot.s3Url of
                Just url ->
                    url
                Nothing ->
                    -- Extract filename from filepath and construct API URL
                    let
                        filename =
                            screenshot.filepath
                                |> String.split "/"
                                |> List.reverse
                                |> List.head
                                |> Maybe.withDefault "screenshot.png"
                    in
                    "/api/screenshots/" ++ filename
    in
    div [ class "space-y-4" ]
        [ -- Main Image Display
          case currentScreenshot of
            Just screenshot ->
                div [ class "relative bg-gray-100 dark:bg-gray-900 rounded-lg overflow-hidden" ]
                    [ img
                        [ Html.Attributes.src (screenshotUrl screenshot)
                        , Html.Attributes.alt ("Screenshot - " ++ screenshot.context)
                        , class "w-full h-auto"
                        ]
                        []
                    , div [ class "absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/70 to-transparent p-4" ]
                        [ div [ class "flex items-center justify-between text-white" ]
                            [ div [ class "space-y-1" ]
                                [ p [ class "text-sm font-medium" ] [ text (String.toUpper screenshot.context) ]
                                , p [ class "text-xs opacity-90" ] [ text screenshot.timestamp ]
                                , p [ class "text-xs opacity-75" ]
                                    [ text (String.fromInt screenshot.width ++ " × " ++ String.fromInt screenshot.height) ]
                                ]
                            , div [ class "flex items-center gap-2" ]
                                [ button
                                    [ class "p-2 bg-white/20 hover:bg-white/30 rounded-lg transition-colors disabled:opacity-50"
                                    , onClick PreviousScreenshot
                                    , Html.Attributes.disabled (currentIndex == 0)
                                    ]
                                    [ text "←" ]
                                , span [ class "text-sm font-medium" ]
                                    [ text (String.fromInt (currentIndex + 1) ++ " / " ++ String.fromInt (List.length screenshots)) ]
                                , button
                                    [ class "p-2 bg-white/20 hover:bg-white/30 rounded-lg transition-colors disabled:opacity-50"
                                    , onClick NextScreenshot
                                    , Html.Attributes.disabled (currentIndex >= List.length screenshots - 1)
                                    ]
                                    [ text "→" ]
                                ]
                            ]
                        ]
                    ]

            Nothing ->
                div [ class "text-center py-8 text-gray-500 dark:text-gray-400" ]
                    [ text "No screenshot selected" ]

        -- Thumbnail Strip
        , div [ class "flex gap-2 overflow-x-auto pb-2" ]
            (List.indexedMap
                (\index screenshot ->
                    button
                        [ class
                            ("flex-shrink-0 relative rounded-lg overflow-hidden border-2 transition-all " ++
                                if index == currentIndex then
                                    "border-blue-500 dark:border-blue-400 shadow-lg"
                                else
                                    "border-transparent hover:border-gray-300 dark:hover:border-gray-600"
                            )
                        , onClick (SelectScreenshot index)
                        ]
                        [ img
                            [ Html.Attributes.src (screenshotUrl screenshot)
                            , Html.Attributes.alt ("Thumbnail " ++ String.fromInt (index + 1))
                            , class "w-32 h-auto"
                            ]
                            []
                        , div [ class "absolute bottom-0 left-0 right-0 bg-black/60 px-2 py-1" ]
                            [ p [ class "text-xs text-white truncate" ] [ text screenshot.context ]
                            ]
                        ]
                )
                screenshots
            )
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


-- Helper function to convert screenshot paths to API URLs
screenshotToUrl : ScreenshotInfo -> String
screenshotToUrl screenshot =
    case screenshot.s3Url of
        Just url ->
            url

        Nothing ->
            -- Extract filename from local path and use API endpoint
            let
                filename =
                    screenshot.filepath
                        |> String.split "/"
                        |> List.reverse
                        |> List.head
                        |> Maybe.withDefault ""
            in
            "/api/screenshots/" ++ filename


viewSideBySide : ScreenshotInfo -> List ScreenshotInfo -> Html Msg
viewSideBySide current allScreenshots =
    let
        previous =
            List.head allScreenshots

        imageUrl =
            screenshotToUrl current

        prevUrl =
            previous
                |> Maybe.map screenshotToUrl
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
            screenshotToUrl current

        prevUrl =
            previous
                |> Maybe.map screenshotToUrl
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
            screenshotToUrl current
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
            screenshotToUrl screenshot
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


scoreColorClass : Int -> String
scoreColorClass score =
    if score >= 80 then
        "text-green-600 dark:text-green-400"

    else if score >= 50 then
        "text-yellow-600 dark:text-yellow-400"

    else
        "text-red-600 dark:text-red-400"


statusBadgeClass : String -> String
statusBadgeClass status =
    case String.toLower status of
        "passed" ->
            "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"

        "failed" ->
            "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"

        "running" ->
            "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"

        _ ->
            "bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200"


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


{-| Task 7: Test History with Sorting, Filtering, and Pagination -}
viewTestHistory : Model -> Html Msg
viewTestHistory model =
    let
        history =
            model.testHistory

        -- Subtask 3: Apply filters and search
        filteredReports =
            history.reports
                |> filterByStatus history.statusFilter
                |> filterByUrlSearch history.urlSearchQuery
                |> sortReports history.sortBy history.sortOrder

        -- Subtask 4: Pagination
        totalFiltered =
            List.length filteredReports

        startIndex =
            (history.currentPage - 1) * history.itemsPerPage

        paginatedReports =
            filteredReports
                |> List.drop startIndex
                |> List.take history.itemsPerPage
    in
    div [ class "space-y-6" ]
        [ h2 [ class "text-3xl font-bold text-gray-900" ] [ text "Test History" ]
        , -- Subtask 3: Filtering and Sorting Controls
          viewHistoryControls history
        , -- Loading/Error States
          if history.loading then
            div [ class "flex items-center justify-center py-12" ]
                [ div [ class "text-lg text-gray-600" ] [ text "Loading test history..." ]
                ]

          else
            case history.error of
                Just errorMsg ->
                    div [ class "bg-red-50 border border-red-200 rounded-lg p-4 text-red-800" ]
                        [ text ("Error: " ++ errorMsg) ]

                Nothing ->
                    if List.isEmpty filteredReports then
                        div [ class "bg-gray-50 border border-gray-200 rounded-lg p-8 text-center text-gray-600" ]
                            [ text "No tests found matching your filters." ]

                    else
                        div [ class "space-y-4" ]
                            [ -- Subtask 2: History Table
                              viewHistoryTable paginatedReports history
                            , -- Subtask 4: Pagination Controls
                              viewHistoryPagination history totalFiltered
                            ]
        ]


{-| Subtask 3: Filter and Sort Controls -}
viewHistoryControls : TestHistoryState -> Html Msg
viewHistoryControls history =
    div [ class "bg-white rounded-lg shadow-md border border-gray-200 p-6 space-y-4" ]
        [ div [ class "flex flex-col md:flex-row md:items-center md:justify-between gap-4" ]
            [ -- URL Search
              div [ class "flex-1" ]
                [ label [ class "block text-sm font-medium text-gray-700 mb-2" ] [ text "Search URL:" ]
                , input
                    [ type_ "text"
                    , placeholder "Filter by game URL..."
                    , value history.urlSearchQuery
                    , onInput UpdateUrlSearch
                    , class "w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                    ]
                    []
                ]
            , -- Status Filter
              div [ class "flex-1" ]
                [ label [ class "block text-sm font-medium text-gray-700 mb-2" ] [ text "Status:" ]
                , select
                    [ onInput UpdateStatusFilter
                    , class "w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                    ]
                    [ option [ value "" ] [ text "All" ]
                    , option [ value "completed" ] [ text "Completed" ]
                    , option [ value "failed" ] [ text "Failed" ]
                    , option [ value "running" ] [ text "Running" ]
                    ]
                ]
            ]
        , -- Sorting Controls
          div [ class "flex flex-col md:flex-row md:items-center md:justify-between gap-4 pt-4 border-t border-gray-200" ]
            [ div [ class "flex items-center gap-3" ]
                [ label [ class "text-sm font-medium text-gray-700" ] [ text "Sort by:" ]
                , div [ class "flex gap-2" ]
                    [ viewSortButton history.sortBy SortByTimestamp "Timestamp"
                    , viewSortButton history.sortBy SortByScore "Score"
                    , viewSortButton history.sortBy SortByDuration "Duration"
                    , viewSortButton history.sortBy SortByStatus "Status"
                    ]
                ]
            , button
                [ class "px-4 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg transition-colors duration-200"
                , onClick ToggleSortOrder
                ]
                [ text
                    (case history.sortOrder of
                        Ascending ->
                            "↑ Ascending"

                        Descending ->
                            "↓ Descending"
                    )
                ]
            ]
        ]


viewSortButton : SortField -> SortField -> String -> Html Msg
viewSortButton currentSort field label =
    button
        [ class
            (if currentSort == field then
                "px-3 py-1.5 bg-blue-600 text-white font-medium rounded-lg shadow-sm transition-colors duration-200"

             else
                "px-3 py-1.5 bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium rounded-lg transition-colors duration-200"
            )
        , onClick (ChangeSortField field)
        ]
        [ text label ]


{-| Subtask 2: History Table -}
viewHistoryTable : List ReportSummary -> TestHistoryState -> Html Msg
viewHistoryTable reports history =
    div [ class "bg-white rounded-lg shadow-md border border-gray-200 overflow-hidden" ]
        [ table [ class "min-w-full divide-y divide-gray-200" ]
            [ thead [ class "bg-gray-50" ]
                [ tr []
                    [ th [ class "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider" ] [ text "Timestamp" ]
                    , th [ class "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider" ] [ text "Game URL" ]
                    , th [ class "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider" ] [ text "Status" ]
                    , th [ class "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider" ] [ text "Score" ]
                    , th [ class "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider" ] [ text "Duration" ]
                    , th [ class "px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider" ] [ text "Actions" ]
                    ]
                ]
            , tbody [ class "bg-white divide-y divide-gray-200" ]
                (List.map viewHistoryRow reports)
            ]
        ]


viewHistoryRow : ReportSummary -> Html Msg
viewHistoryRow report =
    tr
        [ class "hover:bg-gray-50 cursor-pointer transition-colors duration-150"
        , onClick (NavigateToReport report.reportId)
        ]
        [ td [ class "px-6 py-4 whitespace-nowrap text-sm text-gray-900" ] [ text (formatTimestamp report.timestamp) ]
        , td [ class "px-6 py-4 text-sm text-gray-900" ] [ text (truncateUrl report.gameUrl 50) ]
        , td [ class "px-6 py-4 whitespace-nowrap" ]
            [ span
                [ class
                    (case report.status of
                        "completed" ->
                            "px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800"

                        "failed" ->
                            "px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full bg-red-100 text-red-800"

                        "running" ->
                            "px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full bg-blue-100 text-blue-800"

                        _ ->
                            "px-2 py-1 inline-flex text-xs leading-5 font-semibold rounded-full bg-gray-100 text-gray-800"
                    )
                ]
                [ text (String.toUpper report.status) ]
            ]
        , td [ class "px-6 py-4 whitespace-nowrap text-sm text-gray-900" ]
            [ case report.overallScore of
                Just score ->
                    span
                        [ class
                            (if score >= 80 then
                                "font-semibold text-green-600"

                             else if score >= 50 then
                                "font-semibold text-yellow-600"

                             else
                                "font-semibold text-red-600"
                            )
                        ]
                        [ text (String.fromInt score ++ "/100") ]

                Nothing ->
                    span [ class "text-gray-400" ] [ text "N/A" ]
            ]
        , td [ class "px-6 py-4 whitespace-nowrap text-sm text-gray-900" ] [ text (formatDuration report.duration) ]
        , td [ class "px-6 py-4 whitespace-nowrap text-sm font-medium" ]
            [ button
                [ class "text-blue-600 hover:text-blue-900 transition-colors duration-150"
                , onClick (NavigateToReport report.reportId)
                ]
                [ text "View Report" ]
            ]
        ]


{-| Subtask 4: Pagination Controls -}
viewHistoryPagination : TestHistoryState -> Int -> Html Msg
viewHistoryPagination history totalItems =
    let
        totalPages =
            ceiling (toFloat totalItems / toFloat history.itemsPerPage)

        startItem =
            (history.currentPage - 1) * history.itemsPerPage + 1

        endItem =
            Basics.min (history.currentPage * history.itemsPerPage) totalItems

        canGoPrevious =
            history.currentPage > 1

        canGoNext =
            history.currentPage < totalPages
    in
    div [ class "bg-white rounded-lg shadow-md border border-gray-200 px-6 py-4" ]
        [ div [ class "flex flex-col sm:flex-row items-center justify-between gap-4" ]
            [ div [ class "text-sm text-gray-700" ]
                [ text ("Showing " ++ String.fromInt startItem ++ "-" ++ String.fromInt endItem ++ " of " ++ String.fromInt totalItems ++ " tests")
                ]
            , div [ class "flex items-center gap-2" ]
                [ button
                    [ class
                        (if canGoPrevious then
                            "px-3 py-2 bg-white border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors duration-150"

                         else
                            "px-3 py-2 bg-gray-100 border border-gray-300 rounded-lg text-sm font-medium text-gray-400 cursor-not-allowed"
                        )
                    , onClick (ChangeHistoryPage 1)
                    , disabled (not canGoPrevious)
                    ]
                    [ text "First" ]
                , button
                    [ class
                        (if canGoPrevious then
                            "px-3 py-2 bg-white border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors duration-150"

                         else
                            "px-3 py-2 bg-gray-100 border border-gray-300 rounded-lg text-sm font-medium text-gray-400 cursor-not-allowed"
                        )
                    , onClick (ChangeHistoryPage (history.currentPage - 1))
                    , disabled (not canGoPrevious)
                    ]
                    [ text "◀ Previous" ]
                , span [ class "px-4 py-2 text-sm font-medium text-gray-700" ]
                    [ text ("Page " ++ String.fromInt history.currentPage ++ " of " ++ String.fromInt totalPages) ]
                , button
                    [ class
                        (if canGoNext then
                            "px-3 py-2 bg-white border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors duration-150"

                         else
                            "px-3 py-2 bg-gray-100 border border-gray-300 rounded-lg text-sm font-medium text-gray-400 cursor-not-allowed"
                        )
                    , onClick (ChangeHistoryPage (history.currentPage + 1))
                    , disabled (not canGoNext)
                    ]
                    [ text "Next ▶" ]
                , button
                    [ class
                        (if canGoNext then
                            "px-3 py-2 bg-white border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors duration-150"

                         else
                            "px-3 py-2 bg-gray-100 border border-gray-300 rounded-lg text-sm font-medium text-gray-400 cursor-not-allowed"
                        )
                    , onClick (ChangeHistoryPage totalPages)
                    , disabled (not canGoNext)
                    ]
                    [ text "Last" ]
                ]
            ]
        ]


{-| Helper: Filter by status -}
filterByStatus : Maybe String -> List ReportSummary -> List ReportSummary
filterByStatus maybeStatus reports =
    case maybeStatus of
        Nothing ->
            reports

        Just status ->
            List.filter (\r -> r.status == status) reports


{-| Helper: Filter by URL search -}
filterByUrlSearch : String -> List ReportSummary -> List ReportSummary
filterByUrlSearch query reports =
    if String.isEmpty query then
        reports

    else
        let
            lowerQuery =
                String.toLower query
        in
        List.filter (\r -> String.contains lowerQuery (String.toLower r.gameUrl)) reports


{-| Helper: Sort reports -}
sortReports : SortField -> SortOrder -> List ReportSummary -> List ReportSummary
sortReports field order reports =
    let
        sortedReports =
            case field of
                SortByTimestamp ->
                    List.sortBy .timestamp reports

                SortByScore ->
                    List.sortBy (\r -> r.overallScore |> Maybe.withDefault 0) reports

                SortByDuration ->
                    List.sortBy .duration reports

                SortByStatus ->
                    List.sortBy .status reports
    in
    case order of
        Ascending ->
            sortedReports

        Descending ->
            List.reverse sortedReports


{-| Helper: Format timestamp -}
formatTimestamp : String -> String
formatTimestamp timestamp =
    -- Simple formatting, can be enhanced with time library
    String.left 19 timestamp


{-| Helper: Truncate URL -}
truncateUrl : String -> Int -> String
truncateUrl url maxLength =
    if String.length url > maxLength then
        String.left maxLength url ++ "..."

    else
        url


viewNotFound : Html Msg
viewNotFound =
    div [ class "flex items-center justify-center min-h-96" ]
        [ div [ class "text-center space-y-4" ]
            [ div [ class "text-6xl font-bold text-gray-300" ] [ text "404" ]
            , h2 [ class "text-2xl font-semibold text-gray-900" ] [ text "Page Not Found" ]
            , p [ class "text-gray-600" ] [ text "The page you are looking for does not exist." ]
            , a [ href "/", class "inline-flex items-center px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg shadow-sm transition-colors duration-200" ] [ text "Go Home" ]
            ]
        ]


viewFooter : Html Msg
viewFooter =
    footer [ class "bg-slate-900 text-gray-400 mt-auto" ]
        [ div [ class "max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6" ]
            [ p [ class "text-center text-sm" ] [ text "© 2025 DreamUp QA Agent - Automated QA Testing System" ]
            ]
        ]
