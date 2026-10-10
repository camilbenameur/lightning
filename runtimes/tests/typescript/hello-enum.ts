enum Greeting {
  Hello = "Hello, world!",
}

namespace Out {
  export const print = (message: Greeting): void => console.log(message);
}

Out.print(Greeting.Hello);
