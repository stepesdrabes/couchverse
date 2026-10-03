plugins {
    id("couchverse.android.feature")
}

android {
    namespace = "io.stepes.couchverse.ranks"
}

dependencies {
    implementation(libs.androidx.activity.compose)
}
