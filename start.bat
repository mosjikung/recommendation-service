docker-compose up -d
echo "รอ app พร้อม..."
timeout /t 10
docker-compose --profile tools run --rm seeder
docker-compose --profile tools run --rm k6