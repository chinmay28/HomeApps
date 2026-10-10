// The shell's pure logic, as a plain Kotlin/JVM build: no Android SDK needed
// to compile or test it (`gradle -p apps/android/core test`). The app pulls it
// in as an included build.
pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
    }
}

dependencyResolutionManagement {
    repositories {
        mavenCentral()
    }
}

rootProject.name = "core"
