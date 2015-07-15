// See https://developer.mozilla.org/en-US/docs/SpiderMonkey/Shell_global_objects

function uneval(x) { return ''; }

function countHeap() { return 0; }

function assertEq(actual, expected, msg) {
	return true;
}

function tracing(x) { return; }

function version(x) {
	if (typeof x === "undefined") {
		return 1;
	}
	return;
}

