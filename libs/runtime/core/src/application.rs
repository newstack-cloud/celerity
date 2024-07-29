pub struct Application {}

impl Application {
    pub fn new() -> Self {
        Application {}
    }

    pub async fn run(&self) {
        // 1. Load and parse blueprint
        // 2. Determine what kinds of apps to run based on blueprint and env vars.
        //      Can only run one kind of consumer app at a time.
        //      Can run a single HTTP server app.
        //      Can run a single WebSocket server app.
        //      Can run a hybrid app that serves both HTTP and WebSocket.
        // 3. Set up apps with routes and middleware/plugins?!?
        // 4. Start apps in separate tokio tasks.
    }
}
