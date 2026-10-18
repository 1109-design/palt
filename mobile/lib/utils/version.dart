/// version.dart — the PALT mobile app version and where releases live.
///
/// The version comes from pubspec.yaml: CI builds pass it in with
/// `--dart-define=APP_VERSION=<version>`, so there is nothing to bump here.
library;

const _version = String.fromEnvironment('APP_VERSION', defaultValue: '0.0.0');
const String kAppVersion = 'v$_version';

const String kGitHubApiUrl =
    'https://api.github.com/repos/h200137j/palt/releases/latest';

const String kGitHubReleasesUrl =
    'https://github.com/h200137j/palt/releases/latest';

/// Key used when persisting the last-seen version in SharedPreferences.
const String kLastSeenVersionKey = 'palt_last_seen_version';
