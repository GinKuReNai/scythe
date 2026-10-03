export {};
function deadFunction() {}
function liveFunction() {}
liveFunction();
function deadA() { deadB(); }
function deadB() {}
const unusedLiteral = 123;
class UnusedClass {}
