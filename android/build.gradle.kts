plugins {
	id("base")
}

val repoRoot = rootDir.parentFile
val mobilelibAar = repoRoot.resolve("build/android/mobilelib.aar")

tasks.register<Exec>("generateGoMobileAar") {
	group = "build"
	description = "Build gomobile Android bindings for mobilelib."
	workingDir = repoRoot
	commandLine("bash", "./scripts/build-android-mobilelib-aar.sh")
	outputs.file(mobilelibAar)
}
