pluginManagement {
    includeBuild("build-logic")
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

plugins {
    // provisions the JDK 21 pinned in gradle/gradle-daemon-jvm.properties when none is installed
    id("org.gradle.toolchains.foojay-resolver-convention") version "1.0.0"
}

dependencyResolutionManagement {
    repositoriesMode = RepositoriesMode.FAIL_ON_PROJECT_REPOS
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "couchverse"

include(
    ":app",
    ":core",
    ":design",
    ":feature-accounts",
    ":feature-catalog",
    ":feature-couch",
    ":feature-downloads",
    ":feature-playback",
    ":feature-ranks",
    ":feature-settings",
    ":testing",
)
