// Package update keeps Pulsewise current, after yetone/magpie's: a feed
// (latest.json, written by scripts/release.sh next to each release's zip)
// names the newest version and its zip with the zip's SHA-256. A newer one
// is downloaded, checked against that hash, unpacked beside the installed
// bundle, and accepted only if it is signed by the same team as the app
// already installed. Restarting swaps it in (two renames) and opens it
// once this process has exited; quitting swaps it in too.
//
// Only a release updates itself: an app run from its bundle, with a
// version in its Info.plist and a team in its signature. `go run` and the
// -web mode have neither and never ask the feed.
package update
