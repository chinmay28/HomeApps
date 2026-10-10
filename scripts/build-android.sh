#!/usr/bin/env bash
#
# Builds the Android app with its version taken from scripts/version.sh:
# versionName is the calendar version, versionCode the commit count (which
# only grows, as Android requires).
#
#   scripts/build-android.sh [gradle tasks...]    # default: assembleDebug
#
# Needs JDK 17+ and the Android SDK (ANDROID_HOME or apps/android/local.properties).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
name="$("$ROOT/scripts/version.sh")"
code="$("$ROOT/scripts/version.sh" --patch)"
[ "$#" -gt 0 ] || set -- assembleDebug

cd "$ROOT/apps/android"
exec ./gradlew -Phomeapps.versionName="$name" -Phomeapps.versionCode="$code" "$@"
