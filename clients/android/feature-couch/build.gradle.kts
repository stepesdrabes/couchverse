plugins {
    id("couchverse.android.feature")
}

android {
    namespace = "io.stepes.couchverse.couch"
}

dependencies {
    implementation(libs.androidx.core.ktx)
}
