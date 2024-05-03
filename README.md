## Problem:
Removing unused packages from `package.json`, for Node.js projects can be a time-consuming, and error-prone task.
Existing solutions are manual and time-consuming. How about we automate this??
And make it as simple as `go mod tidy`? This is where `depose` comes in.

## Solution:
`Depose` is a tool that automates the task of removing unused packages from your `package.json` file. It is available for installation via npm:

```
npm install depose 
```

After installation, please add it to your package.json's scripts section:
```
"scripts": {
  "depose": "depose"
}
```

Once added, you can run it by the following command:
```
npm run depose 
```

The program will scan all the directories, identify unused packages, and remove the from your package.json file.
An extra file called `oldpackage.json` is created, which is a copy of you previous package.json file. 
Please feel free to compare the changes, and delete the oldpackage.json file once you are satisfied with the changes. 
