const URL = "http://localhost:8080"


const res = await fetch(URL + "/run", {
    method: "POST",
    body: JSON.stringify({
        "language": "python",
        "script": `print('Hello World', 5+5)`,
    }),

    headers: {
        "Content-Type": "application/json"
    }
})

console.log(await res.json(), res.status)