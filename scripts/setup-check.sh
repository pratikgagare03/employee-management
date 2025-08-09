#!/bin/bash

echo "🚀 Employee Management System - Setup Verification"
echo "================================================="

# Check Go version
echo "1. Checking Go version..."
go version

# Check if MySQL is available
echo ""
echo "2. Checking MySQL availability..."
if command -v mysql &> /dev/null; then
    echo "✅ MySQL client found"
    echo 'Just press enter when prompted for password'
    # Try to connect (this will prompt for password)
    sudo mysql -u root -p -e "SELECT 'MySQL connection successful' as status;" 2>/dev/null || echo "❌ MySQL connection failed - please check credentials"
else
    echo "❌ MySQL not found. Please install MySQL server"
fi

# Check if Redis is available
echo ""
echo "3. Checking Redis availability..."
if command -v redis-cli &> /dev/null; then
    redis_status=$(redis-cli ping 2>/dev/null)
    if [ "$redis_status" = "PONG" ]; then
        echo "✅ Redis is running"
    else
        echo "❌ Redis is not running. Start with: sudo systemctl start redis"
    fi
else
    echo "❌ Redis not found. Please install Redis server"
fi

# Check if dependencies are downloaded
echo ""
echo "4. Checking Go dependencies..."
if go mod verify &> /dev/null; then
    echo "✅ All Go dependencies are valid"
else
    echo "❌ Dependencies need to be downloaded. Run: go mod tidy"
fi

# Test compilation
echo ""
echo "5. Testing compilation..."
if go build cmd/main.go &> /dev/null; then
    echo "✅ Application compiles successfully"
    rm -f main  # Clean up binary
else
    echo "❌ Compilation failed. Check the error messages above."
fi

echo ""
echo "🎯 Next Steps:"
echo "1. Make sure MySQL and Redis are running"
echo "2. Create the database: CREATE DATABASE employee_management;"
echo "3. Copy .env.example to .env and configure your database credentials"
echo "4. Run the application: go run cmd/main.go"
echo "5. Test the API: curl http://localhost:8080/api/health"
echo ""
echo "📚 Full documentation available in README.md"
