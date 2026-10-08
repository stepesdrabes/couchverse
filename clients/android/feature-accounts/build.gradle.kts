plugins {
    id("couchverse.android.feature")
}

android {
    namespace = "io.stepes.couchverse.accounts"
}

dependencies {
    // the rank ring and tier names on "Who's watching?"
    implementation(project(":feature-ranks"))
    implementation(libs.androidx.activity.compose)
    // the phone's QR scanner: CameraX frames decoded by ZXing (no Play services needed)
    implementation(libs.androidx.camera.camera2)
    implementation(libs.androidx.camera.lifecycle)
    implementation(libs.androidx.camera.view)
    implementation(libs.zxing.core)
}
