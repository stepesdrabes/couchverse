plugins {
    id("couchverse.android.library")
    id("couchverse.android.compose")
    alias(libs.plugins.roborazzi)
}

android {
    namespace = "io.stepes.couchverse.design"

    sourceSets {
        named("main") {
            // the tokens and string catalogs generated from contract/ by `make contract`
            kotlin.directories += "src/generated/kotlin"
            res.directories += "src/generated/res"
        }
    }
}

dependencies {
    // components render the core's view model types (markdown, accent palettes)
    api(project(":core"))
    api(libs.androidx.compose.ui)
    api(libs.androidx.compose.ui.graphics)
    api(libs.androidx.compose.ui.unit)
    api(libs.androidx.compose.foundation)
    api(libs.androidx.compose.animation)
    api(libs.androidx.compose.material3)
    api(libs.androidx.compose.material.icons.core)
    api(libs.androidx.tv.material)
    api(libs.androidx.tv.foundation)
    api(libs.coil.compose)
    api(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.zxing.core)

    testImplementation(project(":testing"))
    testImplementation(libs.junit)
    testImplementation(libs.kotlin.test.junit)
    testImplementation(libs.robolectric)
    testImplementation(libs.roborazzi)
    testImplementation(libs.roborazzi.compose)
    testImplementation(libs.androidx.compose.ui.test.junit4)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
}
