buildscript {
    dependencies {
        // AGP's built-in Kotlin brings the Kotlin Gradle plugin it was built against; this raises
        // the compiler to the catalog's version
        classpath(libs.kotlin.gradle.plugin)
    }
}

plugins {
    alias(libs.plugins.android.library) apply false
    alias(libs.plugins.kotlin.serialization) apply false
}
