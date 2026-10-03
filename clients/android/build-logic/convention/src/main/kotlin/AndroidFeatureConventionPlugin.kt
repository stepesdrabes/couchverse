import org.gradle.api.Plugin
import org.gradle.api.Project
import org.gradle.api.artifacts.VersionCatalogsExtension
import org.gradle.kotlin.dsl.dependencies
import org.gradle.kotlin.dsl.getByType
import org.gradle.kotlin.dsl.project

/**
 * A `feature-*` module: Compose screens over the core's view models, with phone and TV UI, and
 * JVM tests (Robolectric, Compose UI tests and Roborazzi screenshots).
 */
class AndroidFeatureConventionPlugin : Plugin<Project> {
    override fun apply(target: Project) {
        with(target) {
            pluginManager.apply("couchverse.android.library")
            pluginManager.apply("couchverse.android.compose")
            pluginManager.apply("io.github.takahirom.roborazzi")
            val libs = extensions.getByType<VersionCatalogsExtension>().named("libs")
            fun lib(alias: String) = libs.findLibrary(alias).get()
            dependencies {
                add("api", project(":core"))
                add("api", project(":design"))
                add("implementation", lib("androidx-lifecycle-runtime-compose"))

                add("testImplementation", project(":testing"))
                add("testImplementation", lib("junit"))
                add("testImplementation", lib("kotlin-test-junit"))
                add("testImplementation", lib("robolectric"))
                add("testImplementation", lib("roborazzi"))
                add("testImplementation", lib("roborazzi-compose"))
                add("testImplementation", lib("androidx-compose-ui-test-junit4"))
                add("debugImplementation", lib("androidx-compose-ui-test-manifest"))
            }
        }
    }
}
