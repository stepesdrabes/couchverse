plugins {
    id("couchverse.android.feature")
}

android {
    namespace = "io.stepes.couchverse.playback"
}

dependencies {
    implementation(project(":feature-couch"))
    api(libs.androidx.media3.exoplayer)
    implementation(libs.androidx.media3.exoplayer.hls)
    implementation(libs.androidx.media3.datasource.okhttp)
    implementation(libs.androidx.media3.session)
    implementation(libs.androidx.media3.ui)
    implementation(libs.androidx.activity.compose)

    testImplementation(libs.androidx.media3.test.utils)
    testImplementation(libs.androidx.media3.test.utils.robolectric)
    testImplementation(libs.okhttp.mockwebserver)
}
