const express = require('express');
const PORT = 80;
let app = express();

var responseBody = {
    '/secret': `
        <a href="/">Home</a>
        <a href="/secret2">Click Me</a><br/>
    `,

    '/404': `
        <a href="/">Home</a>
        <a href="/403">Click Me</a><br/>
    `,
    '/': `
        <h1>Home</h1>
        <a href="/test"> Click Me </a><br/>
    `,
    '/test': `
        <h1>Test</h1>
        <a href="/secret"> Click Me </a><br/>
        <a href="/test.png"> Click Me </a><br/>
        <a href="test.png"> Click Me </a><br/>
        <a href="/x/test.png"> Click Me </a><br/>
        <a href="/test/scanonlythis/test1"> Click Me </a>
    `,
    '/test/scanonlythis/test1': `
        <h1>Test</h1>
        <a href="/secret"> Click Me </a><br/>
        <a href="/test/scanonlythis/test2/secret"> Click Me </a>
    `,
    '/test/scanonlythis/test2/secret': `
        <h1>Test</h1>
        <a href="/"> Home </a><br/>
        <a href="/secret"> Click Me </a><br/>
        Current page: <a href="/test/scanonlythis/test2/secret"> Click Me </a>
    `,
};

app.use((req, res, next) => {
    console.log(req.path);
    setTimeout(() => {
        if (responseBody.hasOwnProperty(req.path)) {
            res.setHeader('Content-Type', 'text/html');
            res.end(
                `<html>
                        <head>
                            <title>Test</title>
                        </head>
                        <body>
                            ${responseBody[req.path]}
                        </body>
                </html>`
            );
        } else {
            next();
        }
    }, 100);
});

app.get('/test.png', (req, res) => {
    res.sendFile(__dirname + '/test.png');
});
app.get('x/test.png', (req, res) => {
    res.sendFile(__dirname + '/test.png');
});

app.listen(PORT, function () {
    console.log(`[+] Started on port ${PORT}`);
});
