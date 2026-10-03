plugins {
    id("couchverse.android.feature")
}

android {
    namespace = "io.stepes.couchverse.downloads"
}

dependencies {
    implementation(libs.androidx.work.runtime)

    testImplementation(libs.androidx.work.testing)
    testImplementation(libs.androidx.test.core)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.kotlinx.coroutines.test)
}
