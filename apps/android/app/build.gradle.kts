import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// Release signing comes from the environment (CI secrets); without it a
// release build is simply unsigned.
val keystorePath: String? = System.getenv("HOMEAPPS_KEYSTORE")

android {
    namespace = "io.github.chinmay28.homeapps"
    compileSdk = 36

    defaultConfig {
        applicationId = "io.github.chinmay28.homeapps"
        minSdk = 26
        targetSdk = 36
        // The calendar version and its commit count (scripts/version.sh);
        // the count only grows, which is what versionCode must do.
        versionCode = (findProperty("homeapps.versionCode") as String?)?.toIntOrNull()?.coerceAtLeast(1) ?: 1
        versionName = (findProperty("homeapps.versionName") as String?) ?: "dev"
    }

    signingConfigs {
        if (keystorePath != null) {
            create("release") {
                storeFile = file(keystorePath)
                storePassword = System.getenv("HOMEAPPS_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("HOMEAPPS_KEY_ALIAS")
                keyPassword = System.getenv("HOMEAPPS_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            if (keystorePath != null) signingConfig = signingConfigs.getByName("release")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    compilerOptions { jvmTarget.set(JvmTarget.JVM_17) }
}

dependencies {
    implementation("io.github.chinmay28.homeapps:core")
    implementation("androidx.core:core-ktx:1.16.0")
    implementation("androidx.activity:activity-ktx:1.10.1")
}
