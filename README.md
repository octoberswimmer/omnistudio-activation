# Reactivate OmniStudio OmniScripts and Flex Cards

Port of node [puppeteer script](https://github.com/vlocityinc/vlocity_build/blob/e9b4caa2e5a21ed669aa543a3250c067dc961692/lib/datapacktypes/omniscript.js#L294) to generate LWC components for OmniStudio
OmniScripts and FlexCards to Go and chromedp.

The active [force CLI](https://github.com/ForceCLI/force) login is used, so log
in using `force login` or set your active user using `force active -a
<username>` before running your application.

## Installation

```
$ go install github.com/octoberswimmer/omnistudio-activation
```

## Debugging

To run the application in a non-headless browser:

```
$ env HEADLESS=false go run main.go
```

Enable debug logging by setting the `DEBUG` environment variable to true.

```
$ env DEBUG=true go run main.go
```
