plugins {
    id("couchverse.android.library")
    id("couchverse.android.compose")
}

android {
    namespace = "io.stepes.couchverse.testing"
}

// shared by the JVM tests of the UI modules: screenshot devices, locales, fake images
dependencies {
    api(project(":design"))
    api(libs.junit)
    api(libs.robolectric)
    api(libs.roborazzi)
    api(libs.roborazzi.compose)
    api(libs.androidx.compose.ui.test.junit4)
    api(libs.coil.test)
    api(libs.androidx.test.core)
    implementation(libs.androidx.core.ktx)
}
