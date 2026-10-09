pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "HomeApps"
include(":app")
// The shell's pure logic, testable without the Android SDK; substituted for
// io.github.chinmay28.homeapps:core.
includeBuild("core")
