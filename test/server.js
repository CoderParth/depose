const express = require("express");
const prisma = require("prisma");
const app = express();

// controllers
const productController = require("./controllers/ProductController");

// Middleware
app.use(express.json());

// Routes
app.get("/", (req, res) => {
  res.status(200).json({ message: "A backend API for Product" });
});

// Product
app.use("/products", productController);

// Error handling middleware
app.use((err, req, res, next) => {
  console.error(err);
  res.status(500).json({ error: "Internal Server Error" });
});

module.exports = app;
