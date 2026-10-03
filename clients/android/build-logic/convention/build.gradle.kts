plugins {
    `kotlin-dsl`
}

dependencies {
    // the root build puts the plugins themselves on the classpath (`apply false`)
    compileOnly(libs.android.gradle.plugin)
}

gradlePlugin {
    plugins {
        register("androidLibrary") {
            id = "couchverse.android.library"
            implementationClass = "AndroidLibraryConventionPlugin"
        }
        register("androidCompose") {
            id = "couchverse.android.compose"
            implementationClass = "AndroidComposeConventionPlugin"
        }
        register("androidFeature") {
            id = "couchverse.android.feature"
            implementationClass = "AndroidFeatureConventionPlugin"
        }
    }
}
