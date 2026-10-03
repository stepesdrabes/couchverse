plugins {
    id("couchverse.android.feature")
}

android {
    namespace = "io.stepes.couchverse.settings"
}

dependencies {
    // approving a device reuses the accounts feature's QR scanner
    implementation(project(":feature-accounts"))
}
