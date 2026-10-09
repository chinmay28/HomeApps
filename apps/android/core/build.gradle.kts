plugins {
    kotlin("jvm") version "2.2.0"
}

group = "io.github.chinmay28.homeapps"

kotlin {
    jvmToolchain(17)
}

dependencies {
    testImplementation(kotlin("test"))
}

tasks.test {
    useJUnitPlatform()
}
