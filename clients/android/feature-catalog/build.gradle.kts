plugins {
    id("couchverse.android.feature")
}

android {
    namespace = "io.stepes.couchverse.catalog"
}

dependencies {
    implementation(project(":feature-downloads"))
}
